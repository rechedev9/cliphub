export const STUDIO_CLIPBOARD_CHANNEL = 'cliphub:clipboard-write';

const MAX_CLIPBOARD_TEXT_LENGTH = 512 * 1024;

export interface ClipboardWriteRequest {
  text: string;
}

/** Parses the only clipboard message accepted from the sandboxed preload. */
export function parseClipboardWriteRequest(value: unknown): ClipboardWriteRequest {
  if (!isRecord(value) || Object.keys(value).length !== 1 || typeof value.text !== 'string') {
    throw new Error('invalid clipboard write request');
  }
  if (value.text.length > MAX_CLIPBOARD_TEXT_LENGTH) {
    throw new Error('clipboard text is too large');
  }
  return { text: value.text };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
