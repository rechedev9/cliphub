import { app, dialog, ipcMain } from 'electron';
import { execFile } from 'node:child_process';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { GifTargets, parseReplayGifRequest, STUDIO_REPLAY_GIF_CHANNEL } from './replay-gif-ipc';
import type { ReplayFileFormat } from './replay-gif-ipc';
import type { SaveFileOptions } from './report-pdf-window';

type GifReply = { ok: boolean; token?: string; name?: string; canceled?: boolean; error?: string; copied?: boolean };

const NAMES: Record<ReplayFileFormat, { filter: string; noun: string; folder: 'pictures' | 'videos' }> = {
  gif: { filter: 'GIF', noun: 'el GIF', folder: 'pictures' },
  mp4: { filter: 'Vídeo MP4', noun: 'el MP4', folder: 'videos' },
};

const targets = new GifTargets();
// The next file of a kind opens where the last one went.
const lastFolder: Record<ReplayFileFormat, string | null> = { gif: null, mp4: null };

async function pick(request: { fileName: string; format: ReplayFileFormat }, options: SaveFileOptions): Promise<GifReply> {
  const { fileName, format } = request;
  const win = options.parentWindow();
  const dialogOptions = {
    defaultPath: path.join(lastFolder[format] ?? app.getPath(NAMES[format].folder), fileName),
    filters: [{ name: NAMES[format].filter, extensions: [format] }],
  };
  const target = win === null ? await dialog.showSaveDialog(dialogOptions) : await dialog.showSaveDialog(win, dialogOptions);
  if (target.canceled || !target.filePath) return { ok: false, canceled: true };
  lastFolder[format] = path.dirname(target.filePath);
  return { ok: true, token: targets.add({ path: target.filePath, format }, Date.now()) };
}

async function write(request: { token: string; bytes: Uint8Array; format: ReplayFileFormat }, options: SaveFileOptions): Promise<GifReply> {
  const { token, bytes, format } = request;
  const target = targets.take(token, Date.now());
  if (target === null) return { ok: false, error: `El destino de ${NAMES[format].noun} caducó: expórtalo otra vez.` };
  if (target.format !== format) return { ok: false, error: `No se pudo guardar ${NAMES[target.format].noun}.` };
  try {
    await fs.promises.writeFile(target.path, bytes);
  } catch (err) {
    options.log(`[replay] could not write the ${format}: ${String(err)}\n`);
    return { ok: false, error: `No se pudo guardar ${NAMES[format].noun}.` };
  }
  const copied = format === 'gif' && (await copyFileToClipboard(target.path, options));
  return { ok: true, name: path.basename(target.path), copied };
}

/**
 * Puts the saved GIF on the clipboard as a file, so Ctrl+V drops it animated into a Google Doc.
 * Electron cannot write a file list, Windows PowerShell's Set-Clipboard can; the path travels in the environment.
 */
function copyFileToClipboard(filePath: string, options: SaveFileOptions): Promise<boolean> {
  if (process.platform !== 'win32') return Promise.resolve(false);
  return new Promise((resolve) => {
    execFile(
      'powershell.exe',
      ['-NoProfile', '-NonInteractive', '-Command', 'Set-Clipboard -LiteralPath $env:CLIPHUB_CLIPBOARD_FILE'],
      { env: { ...process.env, CLIPHUB_CLIPBOARD_FILE: filePath }, timeout: 10_000, windowsHide: true },
      (err) => {
        if (err !== null) options.log(`[replay] could not copy the gif to the clipboard: ${String(err)}\n`);
        resolve(err === null);
      },
    );
  });
}

/** Saves the Replay's GIF or MP4: the dialog opens on the click, the bytes arrive once encoded. */
export function registerReplayGifIPC(options: SaveFileOptions): void {
  ipcMain.handle(STUDIO_REPLAY_GIF_CHANNEL, async (event, value: unknown): Promise<GifReply> => {
    if (!options.isTrustedSender(event)) return { ok: false, error: 'Solicitud de guardado rechazada.' };
    let request;
    try {
      request = parseReplayGifRequest(value);
    } catch {
      return { ok: false, error: 'Solicitud de guardado no válida.' };
    }
    return request.action === 'pick' ? pick(request, options) : write(request, options);
  });
}
