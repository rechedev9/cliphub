import { app, dialog, ipcMain } from 'electron';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { GifTargets, parseReplayGifRequest, STUDIO_REPLAY_GIF_CHANNEL } from './replay-gif-ipc';
import type { SaveFileOptions } from './report-pdf-window';

type GifReply = { ok: boolean; token?: string; name?: string; canceled?: boolean; error?: string };

const targets = new GifTargets();
// The next GIF opens where the last one went.
let lastFolder: string | null = null;

async function pick(fileName: string, options: SaveFileOptions): Promise<GifReply> {
  const win = options.parentWindow();
  const dialogOptions = {
    defaultPath: path.join(lastFolder ?? app.getPath('pictures'), fileName),
    filters: [{ name: 'GIF', extensions: ['gif'] }],
  };
  const target = win === null ? await dialog.showSaveDialog(dialogOptions) : await dialog.showSaveDialog(win, dialogOptions);
  if (target.canceled || !target.filePath) return { ok: false, canceled: true };
  lastFolder = path.dirname(target.filePath);
  return { ok: true, token: targets.add(target.filePath, Date.now()) };
}

async function write(token: string, bytes: Uint8Array, options: SaveFileOptions): Promise<GifReply> {
  const file = targets.take(token, Date.now());
  if (file === null) return { ok: false, error: 'El destino del GIF caducó: expórtalo otra vez.' };
  try {
    await fs.promises.writeFile(file, bytes);
    return { ok: true, name: path.basename(file) };
  } catch (err) {
    options.log(`[replay] could not write the GIF: ${String(err)}\n`);
    return { ok: false, error: 'No se pudo guardar el GIF.' };
  }
}

/** Saves the Replay's GIF: the dialog opens on the click, the bytes arrive once encoded. */
export function registerReplayGifIPC(options: SaveFileOptions): void {
  ipcMain.handle(STUDIO_REPLAY_GIF_CHANNEL, async (event, value: unknown): Promise<GifReply> => {
    if (!options.isTrustedSender(event)) return { ok: false, error: 'Solicitud de GIF rechazada.' };
    let request;
    try {
      request = parseReplayGifRequest(value);
    } catch {
      return { ok: false, error: 'Solicitud de GIF no válida.' };
    }
    return request.action === 'pick' ? pick(request.fileName, options) : write(request.token, request.bytes, options);
  });
}
