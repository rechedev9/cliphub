// Desktop wrapper: boot the orchestrator and Next server, then show Studio.

import {
  app,
  BrowserWindow,
  clipboard,
  crashReporter,
  ipcMain,
  powerMonitor,
  screen,
  shell,
  session,
  type IpcMainInvokeEvent,
  type Event as ElectronEvent,
} from 'electron';
import { spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import * as path from 'node:path';
import * as fs from 'node:fs';
import { pathToFileURL } from 'node:url';
import { StreamDownloads } from './stream-downloads';
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
import { bridgeEnvironment } from './bridge-environment';
import { createOrchestratorEnvironment } from './orchestrator-environment';
import { steamEnvironment } from './steam-environment';
import { provisionRuntimeTools, RUNTIME_TOOL_LABELS, type RuntimeToolEnvironment } from './runtime-tools';
import { PINNED_HLAE_TOOL } from './hlae-tool';
import { ProcessExitError, ProcessSession, type LaunchedProcess } from './process-session';
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
  INSTALLER_SPAWN_ARGS,
} from './app-update';
import {
  APP_UPDATE_CHANNEL,
  APP_UPDATE_STATUS_CHANNEL,
  parseAppUpdateRequest,
} from './app-update-ipc';
import { TelemetrySettingsStore } from './telemetry-settings';
import { TelemetryClient, type PublicTelemetryStatus, type TelemetryReleaseConfig } from './telemetry-client';
import { TelemetryJournal } from './telemetry-journal';
import { DiagnosticLogClient } from './diagnostic-log-client';
import { PACKAGED_TELEMETRY_CONFIG } from './telemetry-release';
import {
  parseTelemetryEventRequest,
  STUDIO_TELEMETRY_EVENT_CHANNEL,
} from './telemetry-ipc';
import { readPlaybackInfo, type PlaybackInfoCache } from './playback-diagnostics';
import { isAllowedStudioPermission } from './studio-permission-policy';
import { collectDeviceContext } from './device-context';
import {
  classifyExit,
  clearSessionMarker,
  markSessionShuttingDown,
  recordPreviousSession,
  takeCrashOutput,
  type ExitClass,
} from './crash-monitor';
import { errorScreenHtml, RETRY_URL, SEND_DIAGNOSTIC_URL, type DiagnosticSendState } from './error-screen';

// ClipHub never reads this; drop an inherited operator key before spawning children.
delete process.env.XAI_API_KEY;

// Electron documents GPU feature status as usable only after this event.
let gpuInformationReady = false;
let playbackInfoCache: PlaybackInfoCache | null = null;
app.on('gpu-info-update', () => {
  gpuInformationReady = true;
  playbackInfoCache = null;
});

// Every loopback bind and health check uses this host.
const LOOPBACK_HOST = '127.0.0.1';

// Packaged lock is under canonical appData; dev/e2e stays profile-scoped.
const ownsElectronInstance = requestCanonicalSingleInstanceLock(app);
if (!ownsElectronInstance) {
  app.quit();
  process.exit(0);
}

// Minidumps stay in the local crashDumps dir; only their existence is reported.
crashReporter.start({ uploadToServer: false });

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
let diagnosticLogs: DiagnosticLogClient | null = null;

// This session's studio.log tail, kept in memory: the write stream may not have
// flushed the last lines a child printed before it died.
const RECENT_LOG_LINES = 400;
const RECENT_LOG_LINE_CHARS = 4096;
const recentLog: string[] = [];
function rememberLog(text: string): void {
  const lines = text.split('\n');
  if (lines.at(-1) === '') lines.pop();
  for (const line of lines) recentLog.push(line.slice(0, RECENT_LOG_LINE_CHARS));
  if (recentLog.length > RECENT_LOG_LINES) recentLog.splice(0, recentLog.length - RECENT_LOG_LINES);
}

/** Raw (unfiltered) last lines of this session's log; diagnostic records filter it. */
function recentLogText(maxLines: number): string {
  return recentLog.slice(-maxLines).join('\n');
}

function logLine(text: string): void {
  rememberLog(text);
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
  diagnosticLogs?.recordText(text);
}

function packagedTelemetryConfig(): TelemetryReleaseConfig | null {
  if (!app.isPackaged
    || !PACKAGED_TELEMETRY_CONFIG.endpoint.startsWith('https://')
    || PACKAGED_TELEMETRY_CONFIG.ingestKey.length < 24) return null;
  return PACKAGED_TELEMETRY_CONFIG;
}

const telemetrySettings = new TelemetrySettingsStore(path.join(app.getPath('userData'), 'telemetry.json'));
const diagnosticSessionID = randomUUID();
diagnosticLogs = new DiagnosticLogClient({
  directory: path.join(app.getPath('userData'), 'diagnostic-logs'),
  settings: telemetrySettings, config: packagedTelemetryConfig(), release: app.getVersion(),
  sessionID: diagnosticSessionID, localLog: logLine,
});
const telemetryClient = new TelemetryClient({
  settings: telemetrySettings,
  queuePath: path.join(app.getPath('userData'), 'telemetry-queue.json'),
  release: app.getVersion(),
  config: packagedTelemetryConfig(),
  log: logLine,
  sessionID: diagnosticSessionID,
  captureError: (input) => diagnosticLogs?.record({
    source: input.component === 'electron' ? 'studio' : input.component,
    event: input.name, level: 'error', message: input.message ?? `${input.stage}: ${input.class}`,
    jobID: input.jobID, operation: input.class, occurredAt: input.occurredAt,
  }),
});
const telemetryJournal = new TelemetryJournal({
  client: telemetryClient,
  errorJournalPath: path.join(dataDir, 'obs', 'journal.jsonl'),
  spanJournalPath: path.join(dataDir, 'obs', 'spans.jsonl'),
  cursorPath: path.join(app.getPath('userData'), 'telemetry-cursors.json'),
  log: logLine,
});

process.on('uncaughtExceptionMonitor', (error) => {
  telemetryClient.recordError({
    component: 'electron',
    name: 'process.uncaught_exception',
    stage: 'runtime',
    class: 'uncaught_exception',
    message: error,
  });
});
process.on('unhandledRejection', (reason) => {
  logLine(`[runtime] unhandled rejection: ${String(reason)}\n`);
  telemetryClient.recordError({
    component: 'electron',
    name: 'process.unhandled_rejection',
    stage: 'runtime',
    class: 'unhandled_rejection',
    message: reason,
  });
});

const portsFile = path.join(app.getPath('userData'), 'ports.json');
// Removed on clean quit; a leftover marker at start means the main process died.
const sessionMarkerFile = path.join(app.getPath('userData'), 'session-marker.json');
// The orchestrator's Go runtime appends fatal errors here (debug.SetCrashOutput).
const orchestratorCrashFile = path.join(app.getPath('userData'), 'orchestrator-crash.log');

/** Last lines of this session's studio.log for the error screen, which escapes them. */
function logTail(maxLines = 40): string {
  return recentLogText(maxLines) || '(sin registro)';
}

const BACKEND_CRASH_TAIL_LINES = 40;
const SEND_LOG_TAIL_LINES = 200;
const SEND_FLUSH_CAP_MS = 10_000;
const QUIT_FLUSH_CAP_MS = 2_000;

// Set when Windows ends the session: children then die with shutdown exit codes.
let systemShuttingDown = false;

function noteSystemShutdown(): void {
  if (systemShuttingDown) return;
  systemShuttingDown = true;
  logLine('[runtime] Windows session is ending\n');
  markSessionShuttingDown(sessionMarkerFile);
}

/** Diagnostics may leave the machine: a collector exists and the user said yes. */
function diagnosticsEligible(): boolean {
  const status = telemetryClient.status();
  return status.available && status.enabled && status.noticeAcknowledged;
}

function telemetryStatusResponse(status: PublicTelemetryStatus): unknown {
  return { ...status, sessionId: diagnosticSessionID, logDelivery: diagnosticLogs?.status() };
}

/** The opt-in the web notice and Ajustes perform; the error screen's send button reuses it. */
function enableDiagnostics(): PublicTelemetryStatus {
  telemetryJournal.discardPending();
  const status = telemetryClient.update(true);
  diagnosticLogs?.resetConsent(true);
  diagnosticLogs?.record({ source: 'telemetry', event: 'delivery.enabled', message: 'Diagnostic collection enabled' });
  return status;
}

/**
 * Flushes both channels, bounded by capMS. Returns true when the log spool is
 * empty afterwards, so the caller can tell the user the report arrived.
 */
async function drainDiagnostics(capMS: number): Promise<boolean> {
  const deadline = Date.now() + capMS;
  const drainLogs = async (): Promise<void> => {
    let previous = Number.POSITIVE_INFINITY;
    while (diagnosticLogs !== null && Date.now() < deadline) {
      const pending = diagnosticLogs.status().pendingBytes;
      // No progress means a deferred upload; it retries on its own schedule.
      if (pending === 0 || pending >= previous) return;
      previous = pending;
      await diagnosticLogs.flush();
    }
  };
  let timer: NodeJS.Timeout | undefined;
  const cap = new Promise<void>((resolve) => {
    timer = setTimeout(resolve, capMS);
  });
  await Promise.race([Promise.allSettled([telemetryClient.flush(), drainLogs()]), cap]);
  clearTimeout(timer);
  return (diagnosticLogs?.status().pendingBytes ?? 0) === 0;
}

// Tools resolved by the latest boot; device.context reads their pinned versions.
let lastToolEnvironment: RuntimeToolEnvironment = {};
let deviceContextLine: Promise<string> | null = null;
let deviceContextRecorded = false;

/** One device.context record per session, written only once diagnostics are allowed. */
async function recordDeviceContext(): Promise<void> {
  if (deviceContextRecorded || !diagnosticsEligible()) return;
  deviceContextLine ??= collectDeviceContext({
    dataDir,
    toolsDir: path.join(app.getPath('userData'), 'tools'),
    ffmpegPath: lastToolEnvironment.ZV_FFMPEG_PATH,
    hlaePath: lastToolEnvironment.ZV_HLAE_PATH,
    cachePath: path.join(app.getPath('userData'), 'device-context-cache.json'),
    appVersion: app.getVersion(),
    systemVersion: () => process.getSystemVersion(),
    gpuInfo: () => app.getGPUInfo('basic'),
  });
  let line: string;
  try {
    line = await deviceContextLine;
  } catch (error) {
    deviceContextLine = null;
    logLine(`[runtime] device context unavailable: ${String(error)}\n`);
    return;
  }
  if (deviceContextRecorded || !diagnosticsEligible()) return;
  deviceContextRecorded = true;
  diagnosticLogs?.record({ event: 'device.context', level: 'info', message: line });
}

/** Reports a Go fatal error left by an earlier orchestrator run, then removes the file. */
function recordOrchestratorCrashOutput(): void {
  const excerpt = takeCrashOutput(orchestratorCrashFile);
  if (excerpt === null) return;
  logLine('[runtime] an earlier orchestrator run left a fatal error report\n');
  diagnosticLogs?.record({ event: 'runtime.fatal_previous', level: 'error', message: `source=orchestrator\n${excerpt}` });
}

function recordBootFailure(err: unknown): void {
  telemetryClient.recordError({
    component: 'electron',
    name: 'desktop.boot_failed',
    stage: 'boot',
    class: 'boot_failed',
    message: err,
  });
}

/** A backend child died after boot; the tail was captured when it happened. */
function recordBackendCrash(child: string, exitCode: number | null, exitClass: ExitClass, tail: string): void {
  diagnosticLogs?.record({
    event: 'desktop.backend_crashed',
    level: exitClass === 'shutdown_kill' ? 'info' : 'error',
    message: `child=${child} exit=${exitCode ?? 'none'} exit_class=${exitClass}\n${tail}`,
    exitCode: exitCode ?? undefined,
  });
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
  // Windows log off / shutdown: children exit with session-termination codes.
  win.on('session-end', noteSystemShutdown);
  win.webContents.on('unresponsive', () => {
    logLine('[window] renderer unresponsive\n');
    diagnosticLogs?.record({ event: 'renderer.unresponsive', level: 'warn', message: 'window=main' });
  });
  win.webContents.on('responsive', () => {
    logLine('[window] renderer responsive again\n');
    diagnosticLogs?.record({ event: 'renderer.responsive', level: 'info', message: 'window=main' });
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
    if (url === SEND_DIAGNOSTIC_URL) {
      event.preventDefault();
      void sendErrorScreenDiagnostic();
      return;
    }
    if (url === loadingFileUrl || allowedInternalUrls.has(url) || isLoopbackOrigin(url)) return;
    event.preventDefault();
    if (/^https?:\/\//i.test(url)) shell.openExternal(url);
  });

  win.webContents.on('render-process-gone', (_event, details) => {
    logLine(`[window] render process gone: ${JSON.stringify(details)}\n`);
    const recordRendererGone = (): void => telemetryClient.recordError({
      component: 'renderer',
      name: 'renderer.process_gone',
      stage: 'runtime',
      class: 'process_gone',
      message: `reason=${details.reason} exit=${details.exitCode}`,
    });
    recordRendererGone();
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
      recordRendererGone,
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

interface ErrorScreenState {
  error: unknown;
  title?: string;
  hint?: string;
  /** Records this failure's diagnostics again once the user consents from the screen. */
  report: () => void;
  consented: boolean;
  send: DiagnosticSendState;
}

// The error screen on display, if any; cleared when a boot attempt starts.
let errorScreen: ErrorScreenState | null = null;

/** Renders the fatal-error screen as a data: URL, so it never depends on the servers that just failed or died. */
function showErrorScreen(err: unknown, title?: string, hint?: string, report: () => void = () => {}): void {
  errorScreen = {
    error: err,
    title,
    hint,
    report,
    consented: diagnosticsEligible(),
    send: telemetryClient.status().available ? 'idle' : 'unavailable',
  };
  renderErrorScreen();
}

function renderErrorScreen(): void {
  loadingScreenShowing = false;
  const win = aliveWindow();
  const state = errorScreen;
  if (win === null || state === null) return;
  const html = errorScreenHtml({
    error: state.error,
    title: state.title,
    hint: state.hint,
    logFile,
    logTail: logTail(),
    send: state.send,
    consented: state.consented,
    supportCode: telemetrySettings.get().supportCode,
  });
  const url = 'data:text/html;charset=utf-8,' + encodeURIComponent(html);
  // Only the error screen currently on display is a trusted navigation
  // target; drop whatever the previous error screen (if any) allowed.
  allowedInternalUrls.clear();
  allowedInternalUrls.add(url);
  void win.loadURL(url).catch((loadErr: unknown) => {
    logLine(`[window] could not load error screen: ${String(loadErr)}\n`);
  });
}

/**
 * "Enviar este diagnóstico": the click is the consent. It enables diagnostics
 * the same way the notice does, records this failure and the filtered log
 * tail, and flushes. A user who already consented only gets the flush, since
 * the failure was recorded when it happened.
 */
async function sendErrorScreenDiagnostic(): Promise<void> {
  const state = errorScreen;
  if (state === null || (state.send !== 'idle' && state.send !== 'failed')) return;
  state.send = 'sending';
  renderErrorScreen();
  if (!diagnosticsEligible()) {
    try {
      enableDiagnostics();
    } catch (error) {
      logLine(`[telemetry] could not enable diagnostics from the error screen: ${String(error)}\n`);
      if (errorScreen === state) {
        state.send = 'failed';
        renderErrorScreen();
      }
      return;
    }
    state.report();
    diagnosticLogs?.record({ event: 'desktop.log_tail', level: 'info', message: recentLogText(SEND_LOG_TAIL_LINES) });
  }
  await recordDeviceContext();
  const delivered = await drainDiagnostics(SEND_FLUSH_CAP_MS);
  logLine(`[telemetry] error screen diagnostic ${delivered ? 'delivered' : 'queued'}\n`);
  // A retry may have replaced the screen while the upload ran.
  if (errorScreen !== state) return;
  state.send = delivered ? 'sent' : 'queued';
  renderErrorScreen();
}

interface BootAttempt {
  controller: AbortController;
  processes: ProcessSession;
}

interface BootFailureDetails {
  title?: string;
  hint?: string;
  logLabel?: string;
  /** Records this failure's diagnostics; defaults to the desktop.boot_failed event. */
  report?: () => void;
}

let activeBootAttempt: BootAttempt | null = null;

/** 01 Clips y vídeos is the single door: the hub owns the first-run drop. */
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
  const requestedURL = `${webOrigin}/clips`;
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
  const bootStartedAt = performance.now();
  assertBootAttemptActive(attempt);
  // Reuse the existing window on retry instead of opening another one over the
  // error screen from the failed attempt.
  const existing = aliveWindow();
  const bootWindow = existing ?? createWindow();
  errorScreen = null;
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
  lastToolEnvironment = toolEnv;
  void recordDeviceContext();

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
  // Before the new run reopens the file: whatever is there came from an earlier one.
  recordOrchestratorCrashOutput();
  const orch = attempt.processes.launch(
    'orchestrator',
    orchestratorExe,
    [],
    {
      ...createOrchestratorEnvironment({
        dataDir,
        httpAddress: `${LOOPBACK_HOST}:${orchPort}`,
        musicDir,
        recorderPath: recorderExe,
        overlayRendererPath: process.execPath,
        overlayRendererApp: app.isPackaged ? undefined : app.getAppPath(),
        securityEnvironment: orchestratorSecurityEnvironment(security),
        toolEnvironment: toolEnv,
        steamEnvironment: steamEnvironment(process.env),
        bridgeEnvironment: bridgeEnvironment(process.env),
      }),
      ZV_CRASH_OUTPUT: orchestratorCrashFile,
    },
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

  // A post-boot exit is a backend crash (or a Windows log off), not a boot failure.
  const watchPostBoot = (label: 'orchestrator' | 'web', child: LaunchedProcess): void => {
    attempt.processes.watchUnexpectedExit(child, (err: unknown) => {
      if (quitting || activeBootAttempt !== attempt) return;
      const exitCode = err instanceof ProcessExitError ? err.exitCode : null;
      const exitClass = classifyExit(exitCode, systemShuttingDown);
      const tail = recentLogText(BACKEND_CRASH_TAIL_LINES);
      failBootAttempt(attempt, err, {
        title: 'ClipHub Studio se ha detenido',
        hint: 'El backend se detuvo de forma inesperada. Cierra y vuelve a abrir la app.',
        logLabel: `post-boot crash (${exitClass})`,
        report: () => recordBackendCrash(label, exitCode, exitClass, tail),
      });
    });
  };
  watchPostBoot('orchestrator', orch);
  watchPostBoot('web', web);

  setLoadingStatus('Abriendo la interfaz…');
  allowedInternalUrls.clear();
  await loadStudio(webPort, security.proxyMutationCapability);
  assertBootAttemptActive(attempt);
  telemetryClient.recordSpan({
    component: 'electron',
    name: 'desktop.boot',
    stage: 'boot',
    outcome: 'ok',
    durationMS: performance.now() - bootStartedAt,
  });
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
  const report = details.report ?? ((): void => recordBootFailure(err));
  report();
  if (!quitting) showErrorScreen(err, details.title, details.hint, report);
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
    let request;
    try {
      request = parseStudioSettingsRequest(value);
    } catch {
      return settingsFailure('Solicitud de Ajustes no válida.');
    }
    if (request.action === 'telemetry-status') return telemetryStatusResponse(telemetryClient.status());
    if (request.action === 'telemetry-update') {
      if (!request.enabled) {
        diagnosticLogs?.resetConsent(false);
        try {
          const status = telemetryClient.update(false);
          try {
            telemetryJournal.discardPending();
          } catch (error) {
            logLine(`[telemetry] journal cursor reset deferred: ${String(error)}\n`);
          }
          diagnosticLogs?.resetConsent(false);
          return telemetryStatusResponse(status);
        } catch {
          return settingsFailure('No se pudo guardar la preferencia de diagnósticos.');
        }
      }
      try {
        const status = enableDiagnostics();
        void recordDeviceContext();
        return telemetryStatusResponse(status);
      } catch {
        return settingsFailure('No se pudo guardar la preferencia de diagnósticos.');
      }
    }
    if (request.action === 'playback-info') {
      const read = readPlaybackInfo(
        gpuInformationReady,
        { electron: process.versions.electron, chromium: process.versions.chrome },
        () => ({
          hardwareAccelerationEnabled: app.isHardwareAccelerationEnabled(),
          videoDecodeStatus: app.getGPUFeatureStatus().video_decode,
        }),
        playbackInfoCache,
        Date.now(),
      );
      playbackInfoCache = read.cache;
      return read.info;
    }
    return {
      version: app.getVersion(),
      build: app.isPackaged ? 'production' : 'development',
      electronVersion: process.versions.electron,
      chromiumVersion: process.versions.chrome,
    };
  });
}

function registerStudioTelemetryIPC(): void {
  ipcMain.handle(STUDIO_TELEMETRY_EVENT_CHANNEL, (event, value: unknown): { ok: boolean } => {
    if (!trustedSettingsSender(event)) return { ok: false };
    try {
      const request = parseTelemetryEventRequest(value);
      if (request.kind === 'error') {
        telemetryClient.recordError({
          component: 'renderer',
          name: request.name,
          stage: 'renderer',
          class: 'exception',
          message: request.message,
        });
      } else {
        telemetryClient.recordSpan({
          component: 'renderer',
          name: request.name,
          stage: 'renderer',
          outcome: 'ok',
          durationMS: request.durationMS,
        });
      }
      return { ok: true };
    } catch {
      return { ok: false };
    }
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
    const child = spawn(installerPath, [...INSTALLER_SPAWN_ARGS], {
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
    reportError: (phase, error) => {
      telemetryClient.recordError({
        component: 'electron', name: 'update.failed', stage: 'update', class: phase, message: error,
      });
      void telemetryClient.flush();
    },
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
  const downloads = new StreamDownloads();
  session.defaultSession.on('will-download', (_event, item, contents) => {
    if (contents?.id !== aliveWindow()?.webContents.id) return;
    const key = downloads.key(item.getURL(), activeWebOrigin);
    if (!key) return;
    item.once('done', (_doneEvent, state) => {
      if (state === 'completed' && item.getSavePath()) downloads.completed(key, item.getSavePath());
    });
  });
  ipcMain.handle('cliphub:stream-download', (event, value: unknown): boolean => {
    if (!trustedSettingsSender(event) || typeof value !== 'object' || value === null) return false;
    const request = value as { action?: unknown; url?: unknown };
    if (request.action !== 'status' && request.action !== 'reveal') return false;
    const key = downloads.key(request.url, activeWebOrigin);
    const savedPath = key ? downloads.savedPath(key) : undefined;
    if (!savedPath || !fs.existsSync(savedPath)) return false;
    if (request.action === 'reveal') shell.showItemInFolder(savedPath);
    return true;
  });
  // Electron 43 exposes URL/main-frame details here, but no user-gesture flag.
  // Chromium still requires transient user activation for ordinary DOM fullscreen.
  session.defaultSession.setPermissionRequestHandler(
    (webContents, permission, callback, details) => {
      const win = aliveWindow();
      callback(
        isAllowedStudioPermission({
          permission,
          expectedOrigin: activeWebOrigin,
          expectedWebContentsID: win?.webContents.id ?? null,
          requestingWebContentsID: webContents.id,
          requestingOrigin: details.requestingUrl,
          isMainFrame: details.isMainFrame,
          windowFocused: win?.isFocused() === true,
        }),
      );
    },
  );
  session.defaultSession.setPermissionCheckHandler(
    (webContents, permission, requestingOrigin, details) => {
      const win = aliveWindow();
      return isAllowedStudioPermission({
        permission,
        expectedOrigin: activeWebOrigin,
        expectedWebContentsID: win?.webContents.id ?? null,
        requestingWebContentsID: webContents?.id ?? null,
        requestingOrigin,
        requestingURL: details.requestingUrl,
        isMainFrame: details.isMainFrame,
        windowFocused: win?.isFocused() === true,
      });
    },
  );
  registerStudioSettingsIPC();
  registerStudioTelemetryIPC();
  registerStudioClipboardIPC();
  registerAppUpdateIPC();
  powerMonitor.on('shutdown', noteSystemShutdown);
  telemetryClient.start();
  telemetryJournal.start();
  diagnosticLogs?.start();
  diagnosticLogs?.record({ event: 'desktop.runtime', message: `Studio ${app.getVersion()} platform=${process.platform} arch=${process.arch} electron=${process.versions.electron} chromium=${process.versions.chrome} node=${process.versions.node}` });
  recordPreviousSession({
    markerPath: sessionMarkerFile,
    crashDumpsDir: app.getPath('crashDumps'),
    electronVersion: process.versions.electron,
    record: (input) => diagnosticLogs?.record(input),
    log: logLine,
  });
  runBoot();
});

// GPU, utility and other helper processes; a GPU crash silently disables acceleration.
app.on('child-process-gone', (_event, details) => {
  const type = details.type.toLowerCase().replace(/[^a-z0-9]+/g, '_');
  const message = `type=${type} reason=${details.reason} exit=${details.exitCode}`;
  logLine(`[runtime] child process gone: ${message}\n`);
  if (quitting || details.reason === 'clean-exit') return;
  diagnosticLogs?.record({ event: 'process.gone', level: 'error', message, exitCode: details.exitCode });
});

// Set once the queued diagnostics had their bounded chance to upload.
let quitDrainStarted = false;
let quitDrained = false;

app.on('window-all-closed', () => app.quit());
app.on('before-quit', (event) => {
  quitting = true;
  if (quitDrained) return;
  event.preventDefault();
  if (quitDrainStarted) return;
  quitDrainStarted = true;
  // The quit is intentional from here on, even if an installer kills the
  // process before the drain below finishes.
  clearSessionMarker(sessionMarkerFile);
  telemetryJournal.stop();
  diagnosticLogs?.record({ event: 'desktop.stopping', message: 'Studio shutdown requested' });
  disposeAppUpdate();
  shutdown();
  void drainDiagnostics(QUIT_FLUSH_CAP_MS).finally(() => {
    quitDrained = true;
    telemetryClient.stop();
    diagnosticLogs?.stop();
    app.quit();
  });
});
process.on('exit', shutdown);
