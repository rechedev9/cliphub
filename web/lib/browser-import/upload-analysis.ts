import type { PauseReason } from './import-queue.ts';
import type { ScanSuccess } from './scan-messages.ts';

export type UploadOutcome =
  | { ok: true; matchId: string; created: boolean }
  | { ok: false; error: string; detail?: string; pause?: PauseReason };

/** The workspace a batch started in, set by its first upload and cleared when the queue empties. */
export interface WorkspacePin {
  workspace?: string;
}

export interface UploadInput {
  scan: ScanSuccess;
  fallbackName: string;
  signal: AbortSignal;
  onProgress: (sent: number, total: number) => void;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parseBody(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}

/** Reads the answer of POST /api/library/uploads. Status 0 is a request that never reached ClipMind. */
export function readUploadAnswer(status: number, body: unknown): UploadOutcome {
  if (status === 200 || status === 201) {
    if (isRecord(body) && isRecord(body.match) && typeof body.match.id === 'string') {
      return { ok: true, matchId: body.match.id, created: body.created === true };
    }
    return { ok: false, error: 'Respuesta inesperada de ClipMind.' };
  }
  if (status === 0) return { ok: false, error: 'Sin conexión con ClipMind.', pause: 'offline' };
  if (status === 401) return { ok: false, error: 'Tu sesión ha terminado.', pause: 'auth' };
  if (status === 502 || status === 503 || status === 504) return { ok: false, error: 'ClipMind no responde ahora mismo.', pause: 'unavailable' };
  if (status === 413) return { ok: false, error: 'El análisis de esta demo es demasiado grande para subirlo.' };
  const detail = isRecord(body) && typeof body.error === 'string' && body.error !== '' ? body.error : undefined;
  return { ok: false, error: 'ClipMind no pudo guardar esta partida.', detail };
}

function analysisForm(scan: ScanSuccess, fallbackName: string): FormData {
  const form = new FormData();
  form.set('document', new Blob([scan.document], { type: 'application/json' }), 'document.json');
  form.set('positions', new Blob([new Uint8Array(scan.positions)], { type: 'application/octet-stream' }), 'positions.zvpos');
  form.set('file_name', scan.fileName === '' ? fallbackName : scan.fileName);
  form.set('size_bytes', String(scan.sizeBytes));
  form.set('cs2_build', String(scan.cs2Build));
  return form;
}

/** The id of the session's current workspace in an /api/cm/auth/me answer. */
export function workspaceOf(body: unknown): string | undefined {
  return isRecord(body) && isRecord(body.workspace) && typeof body.workspace.id === 'string' ? body.workspace.id : undefined;
}

/** Pauses instead of filing the demo in another team's library when the session switched workspace. */
export function checkPin(pin: WorkspacePin, current: string | undefined): UploadOutcome | undefined {
  if (current === undefined) return undefined;
  pin.workspace ??= current;
  if (pin.workspace === current) return undefined;
  return { ok: false, error: 'Has cambiado de espacio de trabajo.', pause: 'workspace' };
}

async function currentWorkspace(signal: AbortSignal): Promise<string | undefined> {
  const answer = await fetch('/api/cm/auth/me', { cache: 'no-store', signal }).catch(() => undefined);
  signal.throwIfAborted();
  return answer?.ok === true ? workspaceOf(await answer.json().catch(() => undefined)) : undefined;
}

/** Sends one analysis. XMLHttpRequest, not fetch, because only it reports upload progress in every browser. */
export async function uploadAnalysis(input: UploadInput, pin: WorkspacePin): Promise<UploadOutcome> {
  // The upload skips the proxy that renews the sign-in; a scan can outlast the access token.
  const moved = checkPin(pin, await currentWorkspace(input.signal));
  if (moved !== undefined) return moved;
  const form = analysisForm(input.scan, input.fallbackName);
  return new Promise<UploadOutcome>((resolve, reject) => {
    const request = new XMLHttpRequest();
    const onAbort = (): void => request.abort();
    request.open('POST', '/api/library/uploads');
    request.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) input.onProgress(event.loaded, event.total);
    });
    request.addEventListener('load', () => {
      input.signal.removeEventListener('abort', onAbort);
      resolve(readUploadAnswer(request.status, parseBody(request.responseText)));
    });
    request.addEventListener('error', () => {
      input.signal.removeEventListener('abort', onAbort);
      resolve(readUploadAnswer(0, undefined));
    });
    request.addEventListener('abort', () => {
      input.signal.removeEventListener('abort', onAbort);
      reject(new DOMException('Subida cancelada', 'AbortError'));
    });
    input.signal.addEventListener('abort', onAbort, { once: true });
    request.send(form);
  });
}
