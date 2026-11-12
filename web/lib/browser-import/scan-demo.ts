import { parseProgressMessage, parseResultMessage, type ScanResult } from './scan-messages.ts';

export interface ScanDemoOptions {
  onProgress?: (read: number, total: number) => void;
  signal?: AbortSignal;
}

export function scanDemoInBrowser(file: File, options: ScanDemoOptions = {}): Promise<ScanResult> {
  return new Promise<ScanResult>((resolve, reject) => {
    const id = crypto.randomUUID();
    const worker = new Worker('/wasm/scan-worker.js');
    let settled = false;

    function cleanup(): void {
      worker.terminate();
      if (options.signal !== undefined) options.signal.removeEventListener('abort', onAbort);
    }

    function fail(error: Error): void {
      if (settled) return;
      settled = true;
      cleanup();
      reject(error);
    }

    function finish(result: ScanResult): void {
      if (settled) return;
      settled = true;
      cleanup();
      resolve(result);
    }

    function onAbort(): void {
      fail(new DOMException('Escaneo cancelado', 'AbortError'));
    }

    function onMessage(event: MessageEvent): void {
      const progress = parseProgressMessage(event.data);
      if (progress !== undefined) {
        if (progress.id === id) options.onProgress?.(progress.read, progress.total);
        return;
      }
      const result = parseResultMessage(event.data);
      if (result !== undefined) {
        if (result.id === id) finish(result.result);
        return;
      }
      fail(new Error('El escáner devolvió una respuesta no válida'));
    }

    function onMessageError(): void {
      fail(new Error('El escáner devolvió una respuesta no válida'));
    }

    function onError(event: ErrorEvent): void {
      fail(new Error(event.message || 'Error del escáner'));
    }

    worker.addEventListener('message', onMessage);
    worker.addEventListener('error', onError);
    worker.addEventListener('messageerror', onMessageError);

    if (options.signal !== undefined) {
      if (options.signal.aborted) {
        onAbort();
        return;
      }
      options.signal.addEventListener('abort', onAbort, { once: true });
    }

    worker.postMessage({ id, file });
  });
}
