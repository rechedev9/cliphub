// Desktop wrapper: boot the orchestrator and Next server, then show Studio.

import {
  app,
  BrowserWindow,
  clipboard,
  ipcMain,
  screen,
  shell,
  session,
  type IpcMainInvokeEvent,
  type Event as ElectronEvent,
} from 'electron';
import { spawn } from 'node:child_process';
import * as path from 'node:path';
import * as fs from 'node:fs';
import { pathToFileURL } from 'node:url';
import { escapeHtml } from './escaping';
import {
  createBootSecurityCapabilities,
  installProxyCapabilityCookie,
  orchestratorSecurityEnvironment,
  webSecurityEnvironment,
} from './boot-security';
import {
  isAbortedNavigation,
  isSuccessfulInternalReplacement,
  isSupersededInternalNavigation,
  type SuccessfulNavigationEvidence,
} from './boot-navigation';
import {
  fitWindowStateToWorkAreas,
  validateWindowState,
  type WindowState,
} from './window-state';
import { lastLines } from './log-tail';
import { createOrchestratorEnvironment } from './orchestrator-environment';
import { steamEnvironment } from './steam-environment';
import { provisionRuntimeTools, RUNTIME_TOOL_LABELS } from './runtime-tools';
import { PINNED_HLAE_TOOL } from './hlae-tool';
import { ProcessSession, type LaunchedProcess } from './process-session';
import { waitForDesktopServices } from './service-health';
import { provisionMusicLibrary } from './music-library';
import { allocateStableServicePorts } from './stable-ports';
import { requestCanonicalSingleInstanceLock } from './user-data-lock';
import {
  isTrustedSettingsSender,
  parseStudioSettingsRequest,
  STUDIO_SETTINGS_CHANNEL,
} from './studio-settings-ipc';
import { parseClipboardWriteRequest, STUDIO_CLIPBOARD_CHANNEL } from './clipboard-ipc';
import {
  AppUpdateController,
  createDefaultAppUpdateHost,
} from './app-update';
import {
  APP_UPDATE_CHANNEL,
  APP_UPDATE_STATUS_CHANNEL,
  parseAppUpdateRequest,
} from './app-update-ipc';

// ClipHub never reads this; drop an inherited operator key before spawning children.
delete process.env.XAI_API_KEY;

// Every loopback bind and health check uses this host.
const LOOPBACK_HOST = '127.0.0.1';

// Packaged lock is under canonical appData; dev/e2e stays profile-scoped.
const ownsElectronInstance = requestCanonicalSingleInstanceLock(app);
if (!ownsElectronInstance) {
  app.quit();
  process.exit(0);
}

// Compiled to dist/main.js; bundled files still live one level up.
const appRoot = path.join(__dirname, '..');

/** Packaged: process.resourcesPath. Dev: ./build-resources after assemble. */
function resourcePath(...parts: string[]): string {
  const base = app.isPackaged ? process.resourcesPath : path.join(appRoot, 'build-resources');
  return path.join(base, ...parts);
}

// Spawn the orchestrator directly: killing `zv serve` would orphan the real server.
const orchestratorExe = resourcePath(
  'bin',
  process.platform === 'win32' ? 'zv-orchestrator.exe' : 'zv-orchestrator',
);
const recorderExe = resourcePath(
  'bin',
  process.platform === 'win32' ? 'zv-recorder.exe' : 'zv-recorder',
);
const nextServer = resourcePath('web', 'server.js');
const dataDir = path.join(app.getPath('userData'), 'data');
const musicDir = path.join(dataDir, 'music');

// Packaged stdout is invisible; the error screen shows the tail of this log.
const logFile = path.join(app.getPath('userData'), 'studio.log');
let logStream: fs.WriteStream | null = null;
function logLine(text: string): void {
  process.stdout.write(text);
  try {
    if (!logStream) {
      // Keep one previous log so a crash on the last boot is still reportable.
      try {
        fs.renameSync(logFile, `${logFile}.1`);
      } catch {
        // First launch or rename failed; this run's log is still written below.
      }
      logStream = fs.createWriteStream(logFile, { flags: 'w' });
    }
    logStream.write(text);
  } catch {
    // Logging must never break the app; stdout still has the line in dev.
  }
}

const portsFile = path.join(app.getPath('userData'), 'ports.json');

/** Last lines of studio.log, HTML-escaped for the error screen. */
function logTail(maxLines = 40): string {
  try {
    return escapeHtml(lastLines(fs.readFileSync(logFile, 'utf8'), maxLines));
  } catch {
    return '(sin registro)';
  }
}

let mainWindow: BrowserWindow | null = null;
let activeWebOrigin: string | null = null;
let appUpdate: AppUpdateController | null = null;
let appUpdateCheckTimer: NodeJS.Timeout | null = null;
let appUpdateIntervalTimer: NodeJS.Timeout | null = null;

const APP_UPDATE_START_DELAY_MS = 8_000;
const APP_UPDATE_INTERVAL_MS = 6 * 60 * 60 * 1000;

/** Null if the window was closed or destroyed during an await. */
function aliveWindow(): BrowserWindow | null {
  return mainWindow !== null && !mainWindow.isDestroyed() ? mainWindow : null;
}

// Loopback origins filled once boot() knows the ports.
const allowedOrigins = new Set<string>();

/** True if url's origin is one of the loopback servers we just spawned. */
function isLoopbackOrigin(url: string): boolean {
  try {
    return allowedOrigins.has(new URL(url).origin);
  } catch {
    return false;
  }
}

// loading.html lives at the app root (one level up from dist/).
const loadingHtmlPath = path.join(appRoot, 'loading.html');

// Only this file: URL is allowed under file:/ besides the error screen.
const loadingFileUrl = pathToFileURL(loadingHtmlPath).href;

// At most one main-process data: URL is trusted at a time (the error screen).
const allowedInternalUrls = new Set<string>();

// Error-screen retry href; will-navigate intercepts it and never resolves it.
const RETRY_URL = 'https://retry.cliphub.invalid/';

const windowFile = path.join(app.getPath('userData'), 'window.json');

/** Reads saved window bounds and maximize state, falling back to sane defaults if missing, corrupt, or implausibly small. */
function loadWindowState(): WindowState {
  try {
    return fitWindowStateToWorkAreas(
      validateWindowState(JSON.parse(fs.readFileSync(windowFile, 'utf8'))),
      screen.getAllDisplays().map((display) => display.workArea),
    );
  } catch {
    // Missing or unparseable; validateWindowState(undefined) is the fallback.
    return fitWindowStateToWorkAreas(
      validateWindowState(undefined),
      screen.getAllDisplays().map((display) => display.workArea),
    );
  }
}

/** Best-effort save of the window's current size/position/maximize state so the app reopens where the user left it. */
function saveWindowBounds(): void {
  const win = aliveWindow();
  if (win === null) return;
  try {
    // getNormalBounds: getBounds() while maximized would persist the full screen.
    const bounds = win.getNormalBounds();
    fs.writeFileSync(windowFile, JSON.stringify({ ...bounds, isMaximized: win.isMaximized() }));
  } catch (err) {
    logLine(`[window] could not persist bounds: ${String(err)}\n`);
  }
}

// Guards mainWindow.reload() so a crash-loop in the renderer reloads once
// instead of hammering a dead server forever.
let renderProcessGoneReloaded = false;
let renderProcessGoneResetTimer: NodeJS.Timeout | null = null;

// After this, a later unrelated crash gets its own free reload.
const RENDER_CRASH_RESET_DELAY_MS = 60_000;

function createWindow(): BrowserWindow {
  const { bounds, isMaximized } = loadWindowState();
  const win = new BrowserWindow({
    ...bounds,
    backgroundColor: '#0a0a0a',
    title: 'ClipHub Studio',
    webPreferences: {
      // Leave throttling on when hidden so the GPU can sleep.
      backgroundThrottling: true,
      contextIsolation: true,
      nodeIntegration: false,
      preload: path.join(__dirname, 'preload.js'),
      sandbox: true,
      // Packaged users have no menu; diagnosis is the studio.log tail.
      devTools: !app.isPackaged,
    },
  });
  mainWindow = win;
  // Keep the desktop product name; the page titles itself "ClipHub".
  win.on('page-title-updated', (event) => event.preventDefault());
  win.removeMenu();
  if (isMaximized) win.maximize();
  win.on('close', saveWindowBounds);
  // Clear the ref so aliveWindow() fails closed if boot() is still awaiting.
  win.on('closed', () => {
    mainWindow = null;
  });

  // No popups; non-loopback http(s) opens in the system browser.
  win.webContents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//i.test(url) && !isLoopbackOrigin(url)) shell.openExternal(url);
    return { action: 'deny' };
  });

  // Only loopback, loading.html, and the current error data: URL may navigate.
  win.webContents.on('will-navigate', (event, url) => {
    if (url === RETRY_URL) {
      event.preventDefault();
      retryBoot();
      return;
    }
    if (url === loadingFileUrl || allowedInternalUrls.has(url) || isLoopbackOrigin(url)) return;
    event.preventDefault();
    if (/^https?:\/\//i.test(url)) shell.openExternal(url);
  });

  win.webContents.on('render-process-gone', (_event, details) => {
    logLine(`[window] render process gone: ${JSON.stringify(details)}\n`);
    if (quitting) return;
    if (renderProcessGoneResetTimer) {
      clearTimeout(renderProcessGoneResetTimer);
      renderProcessGoneResetTimer = null;
    }
    if (!renderProcessGoneReloaded) {
      renderProcessGoneReloaded = true;
      const alive = aliveWindow();
      if (alive !== null) alive.reload();
      // Reset after the delay so a later unrelated crash still gets one reload.
      renderProcessGoneResetTimer = setTimeout(() => {
        renderProcessGoneReloaded = false;
        renderProcessGoneResetTimer = null;
      }, RENDER_CRASH_RESET_DELAY_MS);
      return;
    }
    // Second crash in the window: reload did not help; stop looping.
    showErrorScreen(
      `La interfaz se ha bloqueado repetidamente (motivo: ${details.reason}).`,
      'La interfaz se ha bloqueado repetidamente',
      'Cierra ClipHub Studio y vuelve a abrirlo. Si el problema persiste, revisa el registro.',
    );
  });

  return win;
}

// True while the loading.html screen is the thing on screen, so
// setLoadingStatus knows whether its target element can even exist.
let loadingScreenShowing = false;

/** Updates the loading screen's #status line; a silent no-op once we've navigated away from it (real app, or error screen). */
function setLoadingStatus(text: string): void {
  const win = aliveWindow();
  if (win === null || !loadingScreenShowing) return;
  win.webContents
    .executeJavaScript(
      `(() => { const el = document.getElementById('status'); if (el) el.textContent = ${JSON.stringify(text)}; })()`,
    )
    .catch(() => {}); // the page may already be gone; never block boot on this
}

/** Renders the fatal-error screen as a data: URL, so it never depends on the servers that just failed or died. */
function showErrorScreen(err: unknown, title?: string, hint?: string): void {
  loadingScreenShowing = false;
  const win = aliveWindow();
  if (win === null) return;
  const defaultTitle = 'ClipHub Studio no pudo arrancar';
  const defaultHint =
    'Si un antivirus ha bloqueado o puesto en cuarentena archivos de ClipHub, restáuralos y vuelve a abrir la app.';
  const html = `<!doctype html><html><head><meta charset="utf-8">
    <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'unsafe-inline'">
    <style>
      body{font:16px system-ui;background:#0a0a0a;color:#eee;padding:2rem}
      a.retry{display:inline-block;margin-top:1rem;padding:.6rem 1.2rem;background:#22d9ee;color:#04121a;
        font-weight:600;text-decoration:none;border-radius:4px}
    </style></head>
    <body>
      <h2>${escapeHtml(title || defaultTitle)}</h2>
      <p>${escapeHtml(err)}</p>
      <p style="color:#999">${hint || defaultHint} Registro completo: ${escapeHtml(logFile)}</p>
      <a class="retry" href="${RETRY_URL}">Reintentar</a>
      <pre style="background:#111;padding:1rem;overflow:auto;max-height:40vh;font-size:12px">${logTail()}</pre>
    </body></html>`;
  const url = 'data:text/html;charset=utf-8,' + encodeURIComponent(html);
  // Only the error screen currently on display is a trusted navigation
  // target; drop whatever the previous error screen (if any) allowed.
  allowedInternalUrls.clear();
  allowedInternalUrls.add(url);
  void win.loadURL(url).catch((loadErr: unknown) => {
    logLine(`[window] could not load error screen: ${String(loadErr)}\n`);
  });
}

interface BootAttempt {
  controller: AbortController;
  processes: ProcessSession;
}

interface BootFailureDetails {
  title?: string;
  hint?: string;
  logLabel?: string;
}

let activeBootAttempt: BootAttempt | null = null;

/** Inicio is the first-run door; /matches is no longer the shell entry. */
async function loadStudio(webPort: number, proxyMutationCapability: string): Promise<void> {
  loadingScreenShowing = false;
  const win = aliveWindow();
  if (win === null) throw new Error('main window is unavailable');
  const webOrigin = `http://${LOOPBACK_HOST}:${webPort}`;
  await installProxyCapabilityCookie(
    win.webContents.session.cookies,
    webOrigin,
    proxyMutationCapability,
  );
  const requestedURL = `${webOrigin}/onboarding`;
  let observedReplacement: SuccessfulNavigationEvidence | null = null;
  let resolveReplacement: ((evidence: SuccessfulNavigationEvidence) => void) | undefined;
  const replacementCompleted = new Promise<SuccessfulNavigationEvidence>((resolve) => {
    resolveReplacement = resolve;
  });
  const onDidNavigate = (
    _event: ElectronEvent,
    url: string,
    httpResponseCode: number,
  ): void => {
    const evidence = { url, httpResponseCode };
    if (!isSuccessfulInternalReplacement(requestedURL, evidence, webOrigin)) return;
    observedReplacement = evidence;
    resolveReplacement?.(evidence);
  };
  win.webContents.on('did-navigate', onDidNavigate);
  try {
    await win.loadURL(requestedURL);
  } catch (error) {
    if (observedReplacement === null && isAbortedNavigation(error)) {
      observedReplacement = await waitForNavigationEvidence(replacementCompleted, 5_000);
    }
    if (!isSupersededInternalNavigation(error, requestedURL, observedReplacement, webOrigin)) {
      throw error;
    }
  } finally {
    win.webContents.off('did-navigate', onDidNavigate);
  }
}

function waitForNavigationEvidence(
  evidence: Promise<SuccessfulNavigationEvidence>,
  timeoutMs: number,
): Promise<SuccessfulNavigationEvidence | null> {
  return new Promise((resolve) => {
    const timeout = setTimeout(() => resolve(null), timeoutMs);
    void evidence.then((value) => {
      clearTimeout(timeout);
      resolve(value);
    });
  });
}

// Generous overall deadline for each server to answer its health check.
const BOOT_HEALTH_TIMEOUT_MS = 60_000;

async function boot(): Promise<void> {
  if (quitting) return;
  if (activeBootAttempt !== null) throw new Error('cannot start a boot while another attempt is active');
  const attempt: BootAttempt = {
    controller: new AbortController(),
    processes: new ProcessSession({ logLine }),
  };
  activeBootAttempt = attempt;

  try {
    await runBootAttempt(attempt);
  } catch (err) {
    if (quitting || attempt.controller.signal.aborted || activeBootAttempt !== attempt) return;
    failBootAttempt(attempt, err);
  }
}

async function runBootAttempt(attempt: BootAttempt): Promise<void> {
  assertBootAttemptActive(attempt);
  // Reuse the existing window on retry instead of opening another one over the
  // error screen from the failed attempt.
  const existing = aliveWindow();
  const bootWindow = existing ?? createWindow();
  await bootWindow.loadFile(loadingHtmlPath);
  assertBootAttemptActive(attempt);
  loadingScreenShowing = true;
  allowedOrigins.clear();
  allowedInternalUrls.clear();

  // Tracks can land in the background; the API rescans the music dir per request.
  provisionMusicLibrary({
    bundledMusicDir: resourcePath('music'),
    musicDir,
    signal: attempt.controller.signal,
    logLine,
  }).catch((err: unknown) => {
    if (!attempt.controller.signal.aborted) logLine(`[music] provision failed: ${String(err)}\n`);
  });

  setLoadingStatus('Preparando herramientas (solo el primer arranque)…');
  const toolEnv = await provisionRuntimeTools(
    {
      toolsDir: path.join(app.getPath('userData'), 'tools'),
      bundledHLAEArchive: resourcePath('hlae', PINNED_HLAE_TOOL.archiveName),
      logLine,
      signal: attempt.controller.signal,
    },
    (name, detail) =>
      setLoadingStatus(`Preparando ${RUNTIME_TOOL_LABELS[name]}${detail ? ` (${detail})` : ''}…`),
  );
  assertBootAttemptActive(attempt);

  // Probe ports after provisioning; first boot can take minutes.
  setLoadingStatus('Eligiendo puertos libres…');
  const security = createBootSecurityCapabilities();
  const { orchestrator: orchPort, web: webPort } = await allocateStableServicePorts({
    host: LOOPBACK_HOST,
    portsFile,
    logLine,
    signal: attempt.controller.signal,
  });
  assertBootAttemptActive(attempt);
  const orchestratorUrl = `http://${LOOPBACK_HOST}:${orchPort}`;
  activeWebOrigin = `http://${LOOPBACK_HOST}:${webPort}`;
  allowedOrigins.add(`http://${LOOPBACK_HOST}:${orchPort}`);
  allowedOrigins.add(activeWebOrigin);

  setLoadingStatus('Iniciando el orquestador…');
  const orch = attempt.processes.launch(
    'orchestrator',
    orchestratorExe,
    [],
    createOrchestratorEnvironment({
      dataDir,
      httpAddress: `${LOOPBACK_HOST}:${orchPort}`,
      musicDir,
      recorderPath: recorderExe,
      securityEnvironment: orchestratorSecurityEnvironment(security),
      toolEnvironment: toolEnv,
      steamEnvironment: steamEnvironment(process.env),
    }),
  );

  setLoadingStatus('Iniciando el servidor web…');
  const web = attempt.processes.launch('web', process.execPath, [nextServer], {
    ELECTRON_RUN_AS_NODE: '1',
    NODE_ENV: 'production',
    PORT: String(webPort),
    HOSTNAME: LOOPBACK_HOST,
    ORCHESTRATOR_URL: orchestratorUrl,
    NODE_OPTIONS: '--max-old-space-size=256 --max-semi-space-size=8',
    ...webSecurityEnvironment(security),
    // Orchestrator unsets this itself; the Next child would inherit it.
    XAI_API_KEY: undefined,
  });

  // Either child dying is terminal during either health wait. Cancelling the
  // attempt also tears down whichever HTTP poll loses the race.
  const childExited = Promise.race([orch.exited, web.exited]);
  await waitForDesktopServices({
    orchestratorUrl,
    webUrl: `http://${LOOPBACK_HOST}:${webPort}/`,
    timeoutMs: BOOT_HEALTH_TIMEOUT_MS,
    signal: attempt.controller.signal,
    childExited,
  });
  assertBootAttemptActive(attempt);

  const watchPostBoot = (child: LaunchedProcess): void => {
    attempt.processes.watchUnexpectedExit(child, (err: unknown) => {
      if (quitting || activeBootAttempt !== attempt) return;
      failBootAttempt(attempt, err, {
        title: 'ClipHub Studio se ha detenido',
        hint: 'El backend se detuvo de forma inesperada. Cierra y vuelve a abrir la app.',
        logLabel: 'post-boot crash',
      });
    });
  };
  watchPostBoot(orch);
  watchPostBoot(web);

  setLoadingStatus('Abriendo la interfaz…');
  allowedInternalUrls.clear();
  await loadStudio(webPort, security.proxyMutationCapability);
  assertBootAttemptActive(attempt);
  scheduleAppUpdateChecks();
}

function failBootAttempt(attempt: BootAttempt, err: unknown, details: BootFailureDetails = {}): void {
  if (activeBootAttempt !== attempt) return;
  attempt.controller.abort();
  const stopped = attempt.processes.stop();
  if (stopped) activeBootAttempt = null;
  allowedOrigins.clear();
  allowedInternalUrls.clear();
  activeWebOrigin = null;
  logLine(`[boot] ${details.logLabel ?? 'failed'}: ${String(err)}\n`);
  if (!quitting) showErrorScreen(err, details.title, details.hint);
}

function assertBootAttemptActive(attempt: BootAttempt): void {
  if (quitting || attempt.controller.signal.aborted || activeBootAttempt !== attempt) {
    throw new Error('boot attempt cancelled');
  }
}

function stopActiveBootAttempt(): boolean {
  const attempt = activeBootAttempt;
  allowedOrigins.clear();
  allowedInternalUrls.clear();
  activeWebOrigin = null;
  if (attempt === null) return true;
  attempt.controller.abort();
  const stopped = attempt.processes.stop();
  if (stopped && activeBootAttempt === attempt) activeBootAttempt = null;
  return stopped;
}

// Guards against overlapping boot() runs from startup and Retry.
let booting = false;

function runBoot(): void {
  if (booting || quitting) return;
  booting = true;
  boot()
    .catch((err: unknown) => logLine(`[boot] unexpected error: ${String(err)}\n`))
    .finally(() => {
      booting = false;
    });
}

function retryBoot(): void {
  if (booting || quitting) return;
  if (!stopActiveBootAttempt()) {
    logLine('[boot] retry deferred because an existing process tree could not be stopped\n');
    return;
  }
  runBoot();
}

function trustedSettingsSender(event: IpcMainInvokeEvent): boolean {
  const win = aliveWindow();
  const senderFrame = event.senderFrame;
  return isTrustedSettingsSender({
    expectedOrigin: activeWebOrigin,
    expectedWebContentsID: win?.webContents.id ?? null,
    isMainFrame: win !== null && senderFrame !== null && senderFrame === win.webContents.mainFrame,
    senderURL: senderFrame?.url ?? '',
    senderWebContentsID: event.sender.id,
  });
}

function settingsFailure(error: string): { error: string; ok: false } {
  return { error, ok: false };
}

function registerStudioSettingsIPC(): void {
  ipcMain.handle(STUDIO_SETTINGS_CHANNEL, (event, value: unknown): unknown => {
    if (!trustedSettingsSender(event)) return settingsFailure('Solicitud de Ajustes rechazada.');
    try {
      parseStudioSettingsRequest(value);
    } catch {
      return settingsFailure('Solicitud de Ajustes no válida.');
    }
    return {
      version: app.getVersion(),
      build: app.isPackaged ? 'production' : 'development',
      electronVersion: process.versions.electron,
      chromiumVersion: process.versions.chrome,
    };
  });
}

function registerStudioClipboardIPC(): void {
  ipcMain.handle(STUDIO_CLIPBOARD_CHANNEL, (event, value: unknown): { ok: boolean; error?: string } => {
    if (!trustedSettingsSender(event)) return { ok: false, error: 'Solicitud de portapapeles rechazada.' };
    try {
      const request = parseClipboardWriteRequest(value);
      clipboard.writeText(request.text);
      return { ok: true };
    } catch {
      return { ok: false, error: 'Solicitud de portapapeles no válida.' };
    }
  });
}

function spawnVerifiedInstaller(installerPath: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const child = spawn(installerPath, ['/S', '--updated'], {
      detached: true,
      stdio: 'ignore',
      windowsHide: true,
    });
    child.once('error', reject);
    child.once('spawn', () => {
      child.unref();
      resolve();
    });
  });
}

function registerAppUpdateIPC(): void {
  const controller = new AppUpdateController(createDefaultAppUpdateHost({
    currentVersion: app.getVersion(),
    isPackaged: app.isPackaged,
    platform: process.platform,
    updatesDirectory: path.join(app.getPath('userData'), 'updates'),
    spawnInstaller: spawnVerifiedInstaller,
    quitApp: () => app.quit(),
    log: logLine,
  }));
  appUpdate = controller;
  controller.subscribe((status) => {
    const win = aliveWindow();
    if (win === null) return;
    win.webContents.send(APP_UPDATE_STATUS_CHANNEL, status);
  });
  ipcMain.handle(APP_UPDATE_CHANNEL, (event, value: unknown): unknown => {
    if (!trustedSettingsSender(event)) return { ok: false, error: 'Solicitud de actualización rechazada.' };
    try {
      const request = parseAppUpdateRequest(value);
      if (request.action === 'status') return controller.status();
      if (request.action === 'check') {
        void controller.check();
        return { ok: true };
      }
      void controller.install();
      return { ok: true };
    } catch {
      return { ok: false, error: 'Solicitud de actualización no válida.' };
    }
  });
}

function scheduleAppUpdateChecks(): void {
  if (appUpdate === null || !app.isPackaged) return;
  if (appUpdateCheckTimer) clearTimeout(appUpdateCheckTimer);
  if (appUpdateIntervalTimer) clearInterval(appUpdateIntervalTimer);
  appUpdateCheckTimer = setTimeout(() => {
    void appUpdate?.check({ quiet: true });
  }, APP_UPDATE_START_DELAY_MS);
  appUpdateIntervalTimer = setInterval(() => {
    void appUpdate?.check({ quiet: true });
  }, APP_UPDATE_INTERVAL_MS);
  appUpdateCheckTimer.unref();
  appUpdateIntervalTimer.unref();
}

function disposeAppUpdate(): void {
  if (appUpdateCheckTimer) {
    clearTimeout(appUpdateCheckTimer);
    appUpdateCheckTimer = null;
  }
  if (appUpdateIntervalTimer) {
    clearInterval(appUpdateIntervalTimer);
    appUpdateIntervalTimer = null;
  }
  appUpdate?.dispose();
  appUpdate = null;
}

// Prevent crash watchers and retries from fighting an intentional shutdown.
let quitting = false;

function shutdown(): void {
  stopActiveBootAttempt();
}

app.on('second-instance', () => {
  const win = aliveWindow();
  if (win === null) return;
  if (win.isMinimized()) win.restore();
  win.focus();
});

app.whenReady().then(() => {
  // Keep web permissions closed; clipboard writes go through preload IPC.
  session.defaultSession.setPermissionRequestHandler(
    (_webContents, _permission, callback) => callback(false),
  );
  session.defaultSession.setPermissionCheckHandler(() => false);
  registerStudioSettingsIPC();
  registerStudioClipboardIPC();
  registerAppUpdateIPC();
  runBoot();
});

app.on('window-all-closed', () => app.quit());
app.on('before-quit', () => {
  quitting = true;
  disposeAppUpdate();
  shutdown();
});
process.on('exit', shutdown);
