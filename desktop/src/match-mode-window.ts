import { BrowserWindow, ipcMain, shell, type IpcMainInvokeEvent, type WebContents } from 'electron';
import { matchModeUrl, parseMatchModeRequest, STUDIO_MATCH_MODE_CHANNEL } from './match-mode';

/** No popups: a non-loopback http(s) link opens in the system browser. */
export function denyPopups(contents: WebContents, isLoopbackOrigin: (url: string) => boolean): void {
  contents.setWindowOpenHandler(({ url }) => {
    if (/^https?:\/\//i.test(url) && !isLoopbackOrigin(url)) shell.openExternal(url);
    return { action: 'deny' };
  });
}

export interface MatchModeOptions {
  isTrustedSender: (event: IpcMainInvokeEvent) => boolean;
  webOrigin: () => string | null;
  isLoopbackOrigin: (url: string) => boolean;
  backgroundColor: string;
}

let matchWindow: BrowserWindow | null = null;

// A small window for a second monitor: the Tabla and lineups of one map, above the game.
function showMatchWindow(url: string, options: MatchModeOptions): void {
  if (matchWindow !== null && !matchWindow.isDestroyed()) {
    void matchWindow.loadURL(url);
    matchWindow.show();
    return;
  }
  const win = new BrowserWindow({
    width: 460, height: 760, minWidth: 360, minHeight: 420,
    alwaysOnTop: true, backgroundColor: options.backgroundColor, title: 'ClipHub Modo partida',
    webPreferences: { backgroundThrottling: true, contextIsolation: true, nodeIntegration: false, sandbox: true, devTools: false },
  });
  win.setAlwaysOnTop(true, 'screen-saver');
  win.removeMenu();
  win.on('page-title-updated', (event) => event.preventDefault());
  win.on('closed', () => {
    matchWindow = null;
  });
  denyPopups(win.webContents, options.isLoopbackOrigin);
  win.webContents.on('will-navigate', (event, target) => {
    if (!options.isLoopbackOrigin(target)) event.preventDefault();
  });
  matchWindow = win;
  void win.loadURL(url);
}

/** Opens or refreshes the Modo partida window when the main window asks. */
export function registerMatchModeIPC(options: MatchModeOptions): void {
  ipcMain.handle(STUDIO_MATCH_MODE_CHANNEL, (event, value: unknown): { ok: boolean } => {
    const origin = options.webOrigin();
    if (origin === null || !options.isTrustedSender(event)) return { ok: false };
    try {
      showMatchWindow(matchModeUrl(origin, parseMatchModeRequest(value)), options);
      return { ok: true };
    } catch {
      return { ok: false };
    }
  });
}
