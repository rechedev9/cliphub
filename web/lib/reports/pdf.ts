/**
 * Saving a report as PDF goes through the desktop bridge, which renders the
 * exported HTML with Electron's printToPDF and asks where to save it. Outside
 * Studio (a plain browser) there is no bridge and the page prints instead.
 */

export type PdfResult =
  | { readonly kind: 'saved' }
  | { readonly kind: 'canceled' }
  | { readonly kind: 'failed'; readonly message: string }
  | { readonly kind: 'unavailable' };

type SavePdf = (html: string, fileName: string) => Promise<unknown>;

function reportsBridge(value: unknown): SavePdf | null {
  if (typeof value !== 'object' || value === null) return null;
  const savePdf: unknown = Reflect.get(value, 'savePdf');
  if (typeof savePdf !== 'function') return null;
  return (html, fileName) => Promise.resolve(Reflect.apply(savePdf, value, [html, fileName]));
}

function field(value: unknown, name: string): unknown {
  return typeof value === 'object' && value !== null ? Reflect.get(value, name) : undefined;
}

/** Asks the desktop to save the report as a PDF and reports what happened. */
export async function saveReportPdf(html: string, fileName: string, scope: object = globalThis): Promise<PdfResult> {
  const bridge = reportsBridge(Reflect.get(scope, 'cliphubReports'));
  if (bridge === null) return { kind: 'unavailable' };
  const result = await bridge(html, fileName);
  if (field(result, 'ok') === true) return { kind: 'saved' };
  if (field(result, 'canceled') === true) return { kind: 'canceled' };
  const error = field(result, 'error');
  return { kind: 'failed', message: typeof error === 'string' ? error : 'No se pudo guardar el PDF.' };
}
