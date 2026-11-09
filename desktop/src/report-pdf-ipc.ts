import { safeFileName } from './safe-file-name.ts';

export const STUDIO_REPORT_PDF_CHANNEL = 'cliphub:report-pdf';

const MAX_REPORT_HTML_LENGTH = 16 * 1024 * 1024;

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
  return safeFileName(raw, { extension: 'pdf', fallback: 'informe' });
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
