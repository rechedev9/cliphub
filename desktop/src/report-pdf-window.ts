import { app, BrowserWindow, dialog, ipcMain, type IpcMainInvokeEvent } from 'electron';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseReportPdfRequest, STUDIO_REPORT_PDF_CHANNEL } from './report-pdf-ipc';

/** What the main process lends the file-saving handlers: who may ask, the dialog's parent and the log. */
export interface SaveFileOptions {
  isTrustedSender: (event: IpcMainInvokeEvent) => boolean;
  parentWindow: () => BrowserWindow | null;
  log: (text: string) => void;
}

// The report renders in its own hidden window: no script, no preload, an
// in-memory session, and no request beyond the report file itself.
const REPORT_PDF_PARTITION = 'cliphub-report-pdf';

// Compares paths, not URL strings: Node and Chromium escape '%', '[' and ']'
// in a temp path differently.
function isReportFile(url: string, file: string): boolean {
  try {
    return path.normalize(fileURLToPath(url)).toLowerCase() === path.normalize(file).toLowerCase();
  } catch {
    return false;
  }
}

async function renderReportPdf(html: string): Promise<Buffer> {
  const dir = await fs.promises.mkdtemp(path.join(app.getPath('temp'), 'cliphub-report-'));
  const file = path.join(dir, 'informe.html');
  await fs.promises.writeFile(file, html, 'utf8');
  const printer = new BrowserWindow({
    show: false,
    webPreferences: {
      contextIsolation: true,
      javascript: false,
      nodeIntegration: false,
      partition: REPORT_PDF_PARTITION,
      sandbox: true,
    },
  });
  try {
    printer.webContents.session.webRequest.onBeforeRequest((details, callback) => {
      callback({ cancel: !isReportFile(details.url, file) });
    });
    printer.webContents.setWindowOpenHandler(() => ({ action: 'deny' }));
    printer.webContents.on('will-navigate', (event) => event.preventDefault());
    await printer.loadFile(file);
    return await printer.webContents.printToPDF({
      pageSize: 'A4',
      printBackground: true,
      margins: { top: 0.5, bottom: 0.5, left: 0.5, right: 0.5 },
    });
  } finally {
    printer.destroy();
    await fs.promises.rm(dir, { recursive: true, force: true });
  }
}

/** Saves a scouting report as a PDF where the user picks. */
export function registerReportPdfIPC(options: SaveFileOptions): void {
  ipcMain.handle(STUDIO_REPORT_PDF_CHANNEL, async (event, value: unknown): Promise<{ ok: boolean; canceled?: boolean; error?: string }> => {
    if (!options.isTrustedSender(event)) return { ok: false, error: 'Solicitud de PDF rechazada.' };
    let request;
    try {
      request = parseReportPdfRequest(value);
    } catch {
      return { ok: false, error: 'Solicitud de PDF no válida.' };
    }
    const win = options.parentWindow();
    const dialogOptions = {
      defaultPath: path.join(app.getPath('documents'), request.fileName),
      filters: [{ name: 'PDF', extensions: ['pdf'] }],
    };
    const target = win === null ? await dialog.showSaveDialog(dialogOptions) : await dialog.showSaveDialog(win, dialogOptions);
    if (target.canceled || !target.filePath) return { ok: false, canceled: true };
    try {
      const pdf = await renderReportPdf(request.html);
      await fs.promises.writeFile(target.filePath, pdf);
      return { ok: true };
    } catch (err) {
      options.log(`[report] could not write the PDF: ${String(err)}\n`);
      return { ok: false, error: 'No se pudo guardar el PDF.' };
    }
  });
}
