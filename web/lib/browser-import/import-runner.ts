import {
  addFiles,
  clearFinished,
  emptyQueue,
  progress,
  recordWorkspace,
  remove,
  requeue,
  resume,
  retry,
  retryAllFailed,
  scanFailed,
  scanSucceeded,
  schedule,
  uploadFailed,
  uploadSucceeded,
  type Failure,
  type QueueState,
} from './import-queue.ts';
import type { ScanResult, ScanSuccess } from './scan-messages.ts';
import { WORKSPACE_CHANGED, type UploadInput, type UploadOutcome } from './upload-analysis.ts';

export interface ScanOptions {
  onProgress: (read: number, total: number) => void;
  signal: AbortSignal;
}

export interface ImportRunnerDeps {
  scan: (file: File, options: ScanOptions) => Promise<ScanResult>;
  upload: (input: UploadInput) => Promise<UploadOutcome>;
  /** The session's workspace, or undefined when it cannot be read. */
  workspace: (signal?: AbortSignal) => Promise<string | undefined>;
  newId: () => string;
  /** Minimum gap between two progress repaints; status changes repaint at once. */
  progressEveryMs?: number;
}

export interface AddReport {
  added: number;
  unsupported: readonly string[];
  repeated: readonly string[];
}

// Sentences the scanner writes in Spanish (cmd/zv-scan-wasm/open.go) and that the coach can act on.
const SCANNER_REASONS = ['este archivo no es', 'es una demo de cs:go', 'la demo descomprimida', 'la demo está incompleta', 'el archivo comprimido', 'el mapa'];

/** A scan failure as a sentence for the coach, with the scanner's own words kept as detail. */
export function describeScanFailure(error: string): Failure {
  const text = error.trim();
  if (SCANNER_REASONS.some((reason) => text.toLowerCase().startsWith(reason))) {
    return { error: `${text.charAt(0).toUpperCase()}${text.slice(1)}.` };
  }
  return { error: 'No se pudo analizar la demo. Puede estar dañada o no ser de CS2.', detail: text === '' ? undefined : text };
}

function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === 'AbortError';
}

/**
 * Runs the queue: it owns the files, the analyses waiting to upload and the
 * in-flight work, and publishes snapshots for useSyncExternalStore.
 */
export class ImportRunner {
  readonly #deps: ImportRunnerDeps;
  #state: QueueState = emptyQueue;
  #published: QueueState = emptyQueue;
  #timer: ReturnType<typeof setTimeout> | undefined;
  readonly #files = new Map<string, File>();
  readonly #results = new Map<string, ScanSuccess>();
  readonly #inFlight = new Map<string, AbortController>();
  readonly #listeners = new Set<() => void>();

  constructor(deps: ImportRunnerDeps) {
    this.#deps = deps;
  }

  subscribe = (listener: () => void): (() => void) => {
    this.#listeners.add(listener);
    return () => this.#listeners.delete(listener);
  };

  getSnapshot = (): QueueState => this.#published;

  add(files: readonly File[]): AddReport {
    const incoming = files.map((file) => ({ id: this.#deps.newId(), name: file.name, size: file.size, lastModified: file.lastModified }));
    const outcome = addFiles(this.#state, incoming);
    const added = new Set(outcome.added);
    incoming.forEach((entry, index) => {
      const file = files[index];
      if (added.has(entry.id) && file !== undefined) this.#files.set(entry.id, file);
    });
    this.#commit(outcome.state);
    this.#pump();
    if (outcome.added.length > 0) void this.#readWorkspace();
    return { added: outcome.added.length, unsupported: outcome.unsupported, repeated: outcome.repeated };
  }

  retry(id: string): void {
    this.#commit(retry(this.#state, id));
    this.#pump();
  }

  retryAllFailed(): void {
    this.#commit(retryAllFailed(this.#state));
    this.#pump();
  }

  resume(): void {
    this.#commit(resume(this.#state));
    this.#pump();
  }

  remove(id: string): void {
    this.#inFlight.get(id)?.abort();
    this.#forget(id);
    this.#commit(remove(this.#state, id));
    this.#pump();
  }

  clearFinished(): void {
    const next = clearFinished(this.#state);
    const kept = new Set(next.items.map((item) => item.id));
    for (const item of this.#state.items) if (!kept.has(item.id)) this.#forget(item.id);
    this.#commit(next);
  }

  #forget(id: string): void {
    this.#files.delete(id);
    this.#results.delete(id);
    this.#inFlight.delete(id);
  }

  #pump(): void {
    const plan = schedule(this.#state);
    this.#commit(plan.state);
    if (plan.upload !== undefined) void this.#upload(plan.upload);
    if (plan.scan !== undefined) void this.#scan(plan.scan);
  }

  async #scan(id: string): Promise<void> {
    const file = this.#files.get(id);
    if (file === undefined) {
      this.#commit(scanFailed(this.#state, { id, error: 'El archivo ya no está disponible. Vuelve a elegirlo.' }));
      this.#pump();
      return;
    }
    const controller = new AbortController();
    this.#inFlight.set(id, controller);
    const onProgress = (read: number, total: number): void => this.#tick(progress(this.#state, { id, done: read, total }));
    let result: ScanResult;
    try {
      result = await this.#deps.scan(file, { onProgress, signal: controller.signal });
    } catch (error) {
      if (isAbort(error) || controller.signal.aborted) return;
      result = { ok: false, error: error instanceof Error ? error.message : '' };
    }
    if (controller.signal.aborted) return;
    this.#inFlight.delete(id);
    if (result.ok) {
      const next = scanSucceeded(this.#state, { id, sha: result.sha256, map: result.map });
      if (next.items.find((item) => item.id === id)?.status === 'scanned') this.#results.set(id, result);
      this.#commit(next);
    } else {
      this.#commit(scanFailed(this.#state, { id, ...describeScanFailure(result.error) }));
    }
    this.#pump();
  }

  async #upload(id: string): Promise<void> {
    const scan = this.#results.get(id);
    const name = this.#files.get(id)?.name ?? '';
    if (scan === undefined) {
      this.#commit(requeue(this.#state, id));
      this.#pump();
      return;
    }
    const controller = new AbortController();
    this.#inFlight.set(id, controller);
    const onProgress = (sent: number, total: number): void => this.#tick(progress(this.#state, { id, done: sent, total }));
    let outcome: UploadOutcome;
    try {
      outcome = await this.#send(id, { scan, fallbackName: name, signal: controller.signal, onProgress });
    } catch (error) {
      if (isAbort(error) || controller.signal.aborted) return;
      outcome = { ok: false, error: 'No se pudo subir el análisis.', detail: error instanceof Error ? error.message : undefined };
    }
    if (controller.signal.aborted) return;
    this.#inFlight.delete(id);
    if (outcome.ok || outcome.pause === undefined) this.#results.delete(id);
    if (outcome.ok) {
      this.#commit(uploadSucceeded(this.#state, { id, matchId: outcome.matchId, created: outcome.created }));
    } else {
      this.#commit(uploadFailed(this.#state, { id, error: outcome.error, detail: outcome.detail, pause: outcome.pause }));
    }
    this.#pump();
  }

  /** New demos keep the workspace open now, so a later switch cannot send them to another team. */
  async #readWorkspace(): Promise<void> {
    const current = await this.#deps.workspace().catch(() => undefined);
    if (current !== undefined) this.#commit(recordWorkspace(this.#state, current));
  }

  /** Reads the workspace first: it renews the sign-in, which the upload route skips, and stops a demo added in
   * another workspace. ClipMind checks the sent id again, for a switch after this read. */
  async #send(id: string, input: Omit<UploadInput, 'workspace'>): Promise<UploadOutcome> {
    const current = await this.#deps.workspace(input.signal);
    if (current !== undefined) this.#commit(recordWorkspace(this.#state, current));
    const workspace = this.#state.items.find((item) => item.id === id)?.workspace;
    if (current !== undefined && workspace !== current) return WORKSPACE_CHANGED;
    return this.#deps.upload({ ...input, workspace });
  }

  /** A status change: repaint now. */
  #commit(next: QueueState): void {
    if (next === this.#state) return;
    this.#state = next;
    if (this.#timer !== undefined) clearTimeout(this.#timer);
    this.#timer = undefined;
    this.#publish();
  }

  /** A progress tick: repaint at most once per interval, with the latest numbers. */
  #tick(next: QueueState): void {
    if (next === this.#state) return;
    this.#state = next;
    if (this.#timer !== undefined) return;
    this.#timer = setTimeout(() => {
      this.#timer = undefined;
      this.#publish();
    }, this.#deps.progressEveryMs ?? 250);
  }

  #publish(): void {
    if (this.#published === this.#state) return;
    this.#published = this.#state;
    for (const listener of this.#listeners) listener();
  }
}
