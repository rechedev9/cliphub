import { formatBytes } from '../storage.ts';

/** Bytes the browser has handed to the local server so far. */
export type UploadProgress = { loaded: number; total: number; percent: number };

export type UploadFormOptions = {
  url: string;
  body: FormData;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  onProgress?: (progress: UploadProgress) => void;
};

/** Null when the total is unknown, so the caller keeps an indeterminate bar. */
export function uploadProgress(loaded: number, total: number): UploadProgress | null {
  if (!Number.isFinite(total) || total <= 0 || !Number.isFinite(loaded)) return null;
  const sent = Math.min(total, Math.max(0, loaded));
  // Floor: "100 %" must mean every byte left, not 99,6 % rounded up.
  return { loaded: sent, total, percent: Math.floor((sent / total) * 100) };
}

/** "120 MB de 480 MB". */
export function formatUploadBytes(progress: UploadProgress): string {
  return `${formatBytes(progress.loaded)} de ${formatBytes(progress.total)}`;
}

/** True once the body is sent and only the server-side stage is left. */
export function uploadSent(progress: UploadProgress | null): boolean {
  return progress !== null && progress.loaded >= progress.total;
}

function responseHeaders(raw: string): Headers {
  const headers = new Headers();
  for (const line of raw.trim().split(/[\r\n]+/)) {
    const at = line.indexOf(':');
    if (at > 0) headers.append(line.slice(0, at).trim(), line.slice(at + 1).trim());
  }
  return headers;
}

function abortError(signal: AbortSignal): unknown {
  return signal.reason ?? new DOMException('The operation was aborted.', 'AbortError');
}

/**
 * POSTs a multipart body and reports upload progress. `fetch` cannot, so this
 * uses XHR (same origin: cookies and Origin go out as with `fetch`) and hands
 * back a `Response` so callers keep their existing error parsing.
 */
export function uploadForm(options: UploadFormOptions): Promise<Response> {
  const { url, body, headers = {}, signal, onProgress } = options;
  if (typeof XMLHttpRequest === 'undefined') return fetch(url, { method: 'POST', body, headers, signal });
  return new Promise<Response>((resolve, reject) => {
    if (signal?.aborted) {
      reject(abortError(signal));
      return;
    }
    const xhr = new XMLHttpRequest();
    const onAbort = (): void => xhr.abort();
    xhr.open('POST', url);
    for (const [name, value] of Object.entries(headers)) xhr.setRequestHeader(name, value);
    xhr.upload.onprogress = (event) => {
      const progress = event.lengthComputable ? uploadProgress(event.loaded, event.total) : null;
      if (progress) onProgress?.(progress);
    };
    xhr.onload = () => {
      signal?.removeEventListener('abort', onAbort);
      const nullBody = xhr.status === 204 || xhr.status === 205 || xhr.status === 304;
      resolve(new Response(nullBody ? null : xhr.responseText, {
        status: xhr.status,
        statusText: xhr.statusText,
        headers: responseHeaders(xhr.getAllResponseHeaders()),
      }));
    };
    xhr.onerror = () => {
      signal?.removeEventListener('abort', onAbort);
      reject(new TypeError('Failed to fetch'));
    };
    xhr.onabort = () => {
      signal?.removeEventListener('abort', onAbort);
      reject(signal ? abortError(signal) : new DOMException('The operation was aborted.', 'AbortError'));
    };
    signal?.addEventListener('abort', onAbort, { once: true });
    xhr.send(body);
  });
}
