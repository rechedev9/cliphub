export const STUDIO_REPORT_PDF_CHANNEL = 'cliphub:report-pdf';

const MAX_REPORT_HTML_LENGTH = 16 * 1024 * 1024;
const MAX_FILE_STEM_LENGTH = 80;
const FALLBACK_FILE_NAME = 'informe.pdf';

export interface ReportPdfRequest {
  html: string;
  fileName: string;
}

/** Parses the only report-to-PDF message accepted from the sandboxed preload. */
export function parseReportPdfRequest(value: unknown): ReportPdfRequest {
  if (!isRecord(value) || Object.keys(value).length !== 2) {
    throw new Error('invalid report pdf request');
  }
  const { html, fileName } = value;
  if (typeof html !== 'string' || typeof fileName !== 'string' || html.trim() === '') {
    throw new Error('invalid report pdf request');
  }
  if (html.length > MAX_REPORT_HTML_LENGTH) {
    throw new Error('report html is too large');
  }
  return { html, fileName: safePdfFileName(fileName) };
}

/** A bare Windows-safe PDF file name: no folders, no dot segments, no reserved characters. */
export function safePdfFileName(raw: string): string {
  const stem = raw
    .replace(/\.pdf$/i, '')
    .split(/[\\/:*?"<>|\u0000-\u001f]+/)
    .map((part) => part.trim())
    .filter((part) => part !== '' && !/^\.+$/.test(part))
    .join(' ')
    .replace(/\s+/g, ' ')
    .replace(/^\.+/, '')
    .slice(0, MAX_FILE_STEM_LENGTH)
    .trim();
  return stem === '' ? FALLBACK_FILE_NAME : `${stem}.pdf`;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
