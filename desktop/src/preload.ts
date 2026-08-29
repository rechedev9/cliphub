import { contextBridge, ipcRenderer } from 'electron';

// Keep this preload self-contained: sandboxed Electron preloads can import the
// electron module, but must not depend on local CommonJS modules at runtime.
const STUDIO_SETTINGS_CHANNEL = 'cliphub:studio-settings';
const STUDIO_CLIPBOARD_CHANNEL = 'cliphub:clipboard-write';
const STUDIO_UPDATE_CHANNEL = 'cliphub:app-update';
const STUDIO_UPDATE_STATUS_CHANNEL = 'cliphub:app-update-status';
const STUDIO_TELEMETRY_EVENT_CHANNEL = 'cliphub:telemetry-event';

interface PreloadBrowserScope {
  navigator?: {
    userActivation?: {
      isActive?: boolean;
    };
  };
}

contextBridge.exposeInMainWorld('cliphubSettings', {
  getAppInfo: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'app-info' }),
  getTelemetry: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'telemetry-status' }),
  updateTelemetry: (enabled: unknown): Promise<unknown> => ipcRenderer.invoke(
    STUDIO_SETTINGS_CHANNEL,
    { action: 'telemetry-update', enabled },
  ),
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
