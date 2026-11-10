import { contextBridge, ipcRenderer } from 'electron';

// Keep this preload self-contained: sandboxed Electron preloads can import the
// electron module, but must not depend on local CommonJS modules at runtime.
const STUDIO_SETTINGS_CHANNEL = 'cliphub:studio-settings';
const STUDIO_CLIPBOARD_CHANNEL = 'cliphub:clipboard-write';
const STUDIO_REPORT_PDF_CHANNEL = 'cliphub:report-pdf';
const STUDIO_UPDATE_CHANNEL = 'cliphub:app-update';
const STUDIO_UPDATE_STATUS_CHANNEL = 'cliphub:app-update-status';
const STUDIO_TELEMETRY_EVENT_CHANNEL = 'cliphub:telemetry-event';
const STUDIO_MATCH_MODE_CHANNEL = 'cliphub:match-mode';
const STUDIO_REPLAY_GIF_CHANNEL = 'cliphub:replay-gif';
const STUDIO_OPEN_PATH_CHANNEL = 'cliphub:open-path';
const STUDIO_OPEN_PATH_ACK_CHANNEL = 'cliphub:open-path-ack';

interface PreloadBrowserScope {
  navigator?: {
    userActivation?: {
      isActive?: boolean;
    };
  };
}

contextBridge.exposeInMainWorld('cliphubSettings', {
  getAppInfo: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'app-info' }),
  getPlaybackInfo: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'playback-info' }),
  getTelemetry: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'telemetry-status' }),
  updateTelemetry: (enabled: unknown): Promise<unknown> => ipcRenderer.invoke(
    STUDIO_SETTINGS_CHANNEL,
    { action: 'telemetry-update', enabled },
  ),
  getAssistantAccess: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'assistant-status' }),
  updateAssistantAccess: (enabled: unknown): Promise<unknown> => ipcRenderer.invoke(
    STUDIO_SETTINGS_CHANNEL,
    { action: 'assistant-update', enabled },
  ),
  installTeamSpeakPlugin: (): Promise<unknown> => {
    if (!userIsActive()) return Promise.resolve({ ok: false, error: 'El plugin se instala desde un clic.' });
    return ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'teamspeak-plugin-install' });
  },
});

contextBridge.exposeInMainWorld('cliphubDownloads', {
  isSaved: (url: unknown): Promise<boolean> => ipcRenderer.invoke('cliphub:stream-download', { action: 'status', url }),
  reveal: (url: unknown): Promise<boolean> => {
    const scope = globalThis as unknown as PreloadBrowserScope;
    if (scope.navigator?.userActivation?.isActive !== true) return Promise.resolve(false);
    return ipcRenderer.invoke('cliphub:stream-download', { action: 'reveal', url });
  },
});

contextBridge.exposeInMainWorld('cliphubTelemetry', {
  recordError: (value: unknown): Promise<unknown> => ipcRenderer.invoke(
    STUDIO_TELEMETRY_EVENT_CHANNEL,
    value,
  ),
  recordSpan: (value: unknown): Promise<unknown> => ipcRenderer.invoke(
    STUDIO_TELEMETRY_EVENT_CHANNEL,
    value,
  ),
});

contextBridge.exposeInMainWorld('cliphubUpdate', {
  getStatus: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_UPDATE_CHANNEL, { action: 'status' }),
  check: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_UPDATE_CHANNEL, { action: 'check' }),
  install: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_UPDATE_CHANNEL, { action: 'install' }),
  onStatus: (listener: (status: unknown) => void): (() => void) => {
    const wrapped = (_event: unknown, status: unknown): void => {
      listener(status);
    };
    ipcRenderer.on(STUDIO_UPDATE_STATUS_CHANNEL, wrapped);
    return () => {
      ipcRenderer.removeListener(STUDIO_UPDATE_STATUS_CHANNEL, wrapped);
    };
  },
});

contextBridge.exposeInMainWorld('cliphubClipboard', {
  writeText: (text: unknown): Promise<void> => {
    const browserScope = globalThis as unknown as PreloadBrowserScope;
    if (browserScope.navigator?.userActivation?.isActive !== true) {
      return Promise.reject(new Error('clipboard write requires user activation'));
    }
    return ipcRenderer.invoke(STUDIO_CLIPBOARD_CHANNEL, { text }).then((result: unknown) => {
      if (
        typeof result !== 'object'
        || result === null
        || !Object.hasOwn(result, 'ok')
        || (result as { ok?: unknown }).ok !== true
      ) {
        throw new Error('clipboard write rejected');
      }
    });
  },
});

function userIsActive(): boolean {
  const nav: unknown = Reflect.get(globalThis, 'navigator');
  if (typeof nav !== 'object' || nav === null || !('userActivation' in nav)) return false;
  const activation: unknown = nav.userActivation;
  return typeof activation === 'object' && activation !== null && 'isActive' in activation && activation.isActive === true;
}

contextBridge.exposeInMainWorld('cliphubMatchMode', {
  open: (request: unknown): Promise<boolean> => {
    if (!userIsActive()) return Promise.resolve(false);
    return ipcRenderer.invoke(STUDIO_MATCH_MODE_CHANNEL, request).then((result: unknown) =>
      typeof result === 'object' && result !== null && 'ok' in result && result.ok === true);
  },
});

contextBridge.exposeInMainWorld('cliphubReports', {
  savePdf: (html: unknown, fileName: unknown): Promise<unknown> => {
    const browserScope = globalThis as unknown as PreloadBrowserScope;
    if (browserScope.navigator?.userActivation?.isActive !== true) {
      return Promise.resolve({ ok: false, error: 'El PDF se guarda desde un clic.' });
    }
    return ipcRenderer.invoke(STUDIO_REPORT_PDF_CHANNEL, { html, fileName });
  },
});

// The page an assistant asked for, routed in the app; the ack tells main the app took it.
contextBridge.exposeInMainWorld('cliphubNavigation', {
  onOpen: (listener: (path: unknown) => void): (() => void) => {
    const wrapped = (_event: unknown, request: unknown): void => {
      if (typeof request !== 'object' || request === null || !('id' in request) || !('path' in request)) return;
      listener(request.path);
      ipcRenderer.send(STUDIO_OPEN_PATH_ACK_CHANNEL, request.id);
    };
    ipcRenderer.on(STUDIO_OPEN_PATH_CHANNEL, wrapped);
    return () => {
      ipcRenderer.removeListener(STUDIO_OPEN_PATH_CHANNEL, wrapped);
    };
  },
});

// The save dialog opens on the click; the GIF is written to the picked path once it is encoded.
contextBridge.exposeInMainWorld('cliphubReplay', {
  pickGif: (fileName: unknown): Promise<unknown> => {
    if (!userIsActive()) return Promise.resolve({ ok: false, error: 'El GIF se guarda desde un clic.' });
    return ipcRenderer.invoke(STUDIO_REPLAY_GIF_CHANNEL, { action: 'pick', fileName });
  },
  writeGif: (token: unknown, bytes: unknown): Promise<unknown> => ipcRenderer.invoke(STUDIO_REPLAY_GIF_CHANNEL, { action: 'write', token, bytes }),
});
