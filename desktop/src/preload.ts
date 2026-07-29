import { contextBridge, ipcRenderer } from 'electron';

// Keep this preload self-contained: sandboxed Electron preloads can import the
// electron module, but must not depend on local CommonJS modules at runtime.
const STUDIO_SETTINGS_CHANNEL = 'fragforge:studio-settings';
const STUDIO_CLIPBOARD_CHANNEL = 'fragforge:clipboard-write';

interface PreloadBrowserScope {
  navigator?: {
    userActivation?: {
      isActive?: boolean;
    };
  };
}

contextBridge.exposeInMainWorld('fragforgeSettings', {
  getAppInfo: (): Promise<unknown> => ipcRenderer.invoke(STUDIO_SETTINGS_CHANNEL, { action: 'app-info' }),
});

contextBridge.exposeInMainWorld('fragforgeClipboard', {
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
