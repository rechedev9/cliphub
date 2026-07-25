import { contextBridge, ipcRenderer } from 'electron';

// Keep this preload self-contained: sandboxed Electron preloads can import the
// electron module, but must not depend on local CommonJS modules at runtime.
const STUDIO_SETTINGS_CHANNEL = 'fragforge:studio-settings';
const ASSISTANT_CHANNEL = 'fragforge:assistant';
const ASSISTANT_EVENT_CHANNEL = 'fragforge:assistant-event';

contextBridge.exposeInMainWorld('fragforgeSettings', {
  getAppInfo: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'app-info' }),
});

/**
 * The embedded FragForge agent gets this one narrow bridge, never generic IPC or
 * Electron APIs. Main process validation remains the security boundary.
 */
contextBridge.exposeInMainWorld('fragforgeAssistant', {
  status: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'status' }),
  wake: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'wake' }),
  send: (request: unknown): Promise<unknown> => {
    if (typeof request !== 'object' || request === null || Array.isArray(request)) {
      return Promise.reject(new Error('invalid assistant send request'));
    }
    const value = request as { context?: unknown; message?: unknown };
    return ipcRenderer.invoke(ASSISTANT_CHANNEL, {
      action: 'send',
      context: value.context,
      message: value.message,
    });
  },
  cancel: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'cancel' }),
  // This can only request the main-owned native dialog. The renderer never
  // receives or supplies the privileged confirmation result.
  approve: (actionId: unknown): Promise<unknown> =>
    ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'request-approval', actionId }),
  reject: (actionId: unknown): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'reject', actionId }),
  newConversation: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'new' }),
  clearHistory: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'clear' }),
  login: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'login' }),
  logout: (): Promise<unknown> => ipcRenderer.invoke(ASSISTANT_CHANNEL, { action: 'logout' }),
  subscribe: (listener: unknown): (() => void) => {
    if (typeof listener !== 'function') throw new Error('assistant listener must be a function');
    const receive = (_event: Electron.IpcRendererEvent, payload: unknown): void => {
      if (isAssistantEvent(payload)) (listener as (event: unknown) => void)(payload);
    };
    ipcRenderer.on(ASSISTANT_EVENT_CHANNEL, receive);
    return () => ipcRenderer.removeListener(ASSISTANT_EVENT_CHANNEL, receive);
  },
});

function isAssistantEvent(value: unknown): boolean {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
    && (value as { type?: unknown }).type === 'snapshot'
    && typeof (value as { snapshot?: unknown }).snapshot === 'object'
    && (value as { snapshot?: unknown }).snapshot !== null;
}
