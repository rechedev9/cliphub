// The bulk import queue as pure state: one scan at a time (WebAssembly memory) while the previous
// result uploads. Every function returns a new state, or the same object when nothing changed.

export type ItemStatus = 'queued' | 'scanning' | 'scanned' | 'uploading' | 'imported' | 'duplicate' | 'failed';

/** Why the whole queue waits: the session ended, the network dropped, ClipMind is down or the workspace changed. */
export type PauseReason = 'auth' | 'offline' | 'unavailable' | 'workspace';

export interface QueueItem {
  readonly id: string;
  readonly name: string;
  readonly size: number;
  readonly fingerprint: string;
  readonly status: ItemStatus;
  readonly percent: number;
  readonly sha?: string;
  readonly map?: string;
  readonly matchId?: string;
  readonly error?: string;
  readonly detail?: string;
  readonly failedStage?: 'scan' | 'upload';
}

export interface QueueState {
  readonly items: readonly QueueItem[];
  readonly paused?: PauseReason;
}

export interface IncomingFile {
  readonly id: string;
  readonly name: string;
  readonly size: number;
  readonly lastModified: number;
}

export interface AddOutcome {
  readonly state: QueueState;
  readonly added: readonly string[];
  readonly unsupported: readonly string[];
  readonly repeated: readonly string[];
}

export interface Plan {
  readonly state: QueueState;
  readonly scan?: string;
  readonly upload?: string;
}

export interface Failure {
  readonly error: string;
  readonly detail?: string;
}

export interface QueueSummary {
  readonly total: number;
  readonly imported: number;
  readonly duplicate: number;
  readonly failed: number;
  readonly pending: number;
  readonly finished: number;
}

export const emptyQueue: QueueState = { items: [] };

// Results that wait for the upload slot. One keeps the scanner busy without piling analyses in memory.
const MAX_WAITING_UPLOADS = 1;

const DEMO_EXTENSION = /\.(dem|zst|gz)$/i;

/** The extension gate; the scanner checks the bytes, so a renamed file still fails with a reason. */
export function isDemoFileName(name: string): boolean {
  return DEMO_EXTENSION.test(name.trim());
}

function fingerprintOf(file: IncomingFile): string {
  return `${file.name}\u0000${file.size}\u0000${file.lastModified}`;
}

function isPending(status: ItemStatus): boolean {
  return status === 'queued' || status === 'scanning' || status === 'scanned' || status === 'uploading';
}

function isFinished(status: ItemStatus): boolean {
  return status === 'imported' || status === 'duplicate';
}

function patchItem(state: QueueState, id: string, change: (item: QueueItem) => QueueItem): QueueState {
  let changed = false;
  const items = state.items.map((item) => {
    if (item.id !== id) return item;
    const next = change(item);
    if (next !== item) changed = true;
    return next;
  });
  return changed ? { ...state, items } : state;
}

function failed(item: QueueItem, failure: Failure & { stage: 'scan' | 'upload' }): QueueItem {
  return { ...item, status: 'failed', percent: 0, error: failure.error, detail: failure.detail, failedStage: failure.stage };
}

/** Appends new demos in order, skipping other files and a file already in the list. */
export function addFiles(state: QueueState, files: readonly IncomingFile[]): AddOutcome {
  const known = new Set(state.items.map((item) => item.fingerprint));
  const added: QueueItem[] = [];
  const unsupported: string[] = [];
  const repeated: string[] = [];
  for (const file of files) {
    if (!isDemoFileName(file.name)) {
      unsupported.push(file.name);
      continue;
    }
    const fingerprint = fingerprintOf(file);
    if (known.has(fingerprint)) {
      repeated.push(file.name);
      continue;
    }
    known.add(fingerprint);
    added.push({ id: file.id, name: file.name, size: file.size, fingerprint, status: 'queued', percent: 0 });
  }
  const next = added.length === 0 ? state : { ...state, items: [...state.items, ...added] };
  return { state: next, added: added.map((item) => item.id), unsupported, repeated };
}

/** Starts the next upload and the next scan when their slots are free. */
export function schedule(state: QueueState): Plan {
  if (state.paused !== undefined) return { state };
  let next = state;
  let upload: string | undefined;
  let scan: string | undefined;
  if (!next.items.some((item) => item.status === 'uploading')) {
    upload = next.items.find((item) => item.status === 'scanned')?.id;
    if (upload !== undefined) next = patchItem(next, upload, (item) => ({ ...item, status: 'uploading', percent: 0 }));
  }
  const waiting = next.items.filter((item) => item.status === 'scanned').length;
  if (waiting < MAX_WAITING_UPLOADS && !next.items.some((item) => item.status === 'scanning')) {
    scan = next.items.find((item) => item.status === 'queued')?.id;
    if (scan !== undefined) next = patchItem(next, scan, (item) => ({ ...item, status: 'scanning', percent: 0 }));
  }
  return { state: next, scan, upload };
}

function percentOf(done: number, total: number): number {
  if (!(total > 0)) return 0;
  return Math.max(0, Math.min(100, Math.floor((done / total) * 100)));
}

/** A progress tick; the same state comes back when the whole percent did not move. */
export function progress(state: QueueState, tick: { id: string; done: number; total: number }): QueueState {
  const percent = percentOf(tick.done, tick.total);
  return patchItem(state, tick.id, (item) =>
    (item.status === 'scanning' || item.status === 'uploading') && item.percent !== percent ? { ...item, percent } : item,
  );
}

/**
 * A finished scan waits for the upload slot. A demo this session already stored
 * under the same checksum (the .dem and the .dem.zst of one match) skips the upload.
 */
export function scanSucceeded(state: QueueState, scan: { id: string; sha: string; map: string }): QueueState {
  const twin = state.items.find((item) => item.id !== scan.id && item.sha === scan.sha && isFinished(item.status));
  return patchItem(state, scan.id, (item) => {
    if (item.status !== 'scanning') return item;
    if (twin?.matchId !== undefined) return { ...item, status: 'duplicate', percent: 100, sha: scan.sha, map: scan.map, matchId: twin.matchId };
    return { ...item, status: 'scanned', percent: 100, sha: scan.sha, map: scan.map };
  });
}

export function scanFailed(state: QueueState, failure: Failure & { id: string }): QueueState {
  return patchItem(state, failure.id, (item) => (item.status === 'scanning' ? failed(item, { ...failure, stage: 'scan' }) : item));
}

export function uploadSucceeded(state: QueueState, answer: { id: string; matchId: string; created: boolean }): QueueState {
  return patchItem(state, answer.id, (item) =>
    item.status === 'uploading' ? { ...item, status: answer.created ? 'imported' : 'duplicate', percent: 100, matchId: answer.matchId } : item,
  );
}

/**
 * A failure of this file marks it failed. A failure of the connection pauses the
 * queue and puts the analysis back in line, so nothing is lost or scanned twice.
 */
export function uploadFailed(state: QueueState, failure: Failure & { id: string; pause?: PauseReason }): QueueState {
  if (failure.pause === undefined) {
    return patchItem(state, failure.id, (item) => (item.status === 'uploading' ? failed(item, { ...failure, stage: 'upload' }) : item));
  }
  const next = patchItem(state, failure.id, (item) => (item.status === 'uploading' ? { ...item, status: 'scanned', percent: 0 } : item));
  return { ...next, paused: failure.pause };
}

export function resume(state: QueueState): QueueState {
  if (state.paused === undefined) return state;
  return { items: state.items };
}

/** A failed item goes back to its stage; the runner drops a failed upload's analysis, so that demo is read again. */
export function retry(state: QueueState, id: string): QueueState {
  return patchItem(state, id, (item) => {
    if (item.status !== 'failed') return item;
    return {
      id: item.id,
      name: item.name,
      size: item.size,
      fingerprint: item.fingerprint,
      status: item.failedStage === 'upload' ? 'scanned' : 'queued',
      percent: 0,
      sha: item.sha,
      map: item.map,
    };
  });
}

export function retryAllFailed(state: QueueState): QueueState {
  return state.items.reduce((next, item) => (item.status === 'failed' ? retry(next, item.id) : next), state);
}

/** Sends a demo back to be read again, for an analysis the runner no longer holds. */
export function requeue(state: QueueState, id: string): QueueState {
  return patchItem(state, id, (item) => (item.status === 'uploading' ? { ...item, status: 'queued', percent: 0 } : item));
}

export function remove(state: QueueState, id: string): QueueState {
  const items = state.items.filter((item) => item.id !== id);
  return items.length === state.items.length ? state : { ...state, items };
}

export function clearFinished(state: QueueState): QueueState {
  const items = state.items.filter((item) => !isFinished(item.status));
  return items.length === state.items.length ? state : { ...state, items };
}

export function summarize(state: QueueState): QueueSummary {
  let imported = 0;
  let duplicate = 0;
  let failedCount = 0;
  let pending = 0;
  for (const item of state.items) {
    if (item.status === 'imported') imported += 1;
    else if (item.status === 'duplicate') duplicate += 1;
    else if (item.status === 'failed') failedCount += 1;
    else if (isPending(item.status)) pending += 1;
  }
  return {
    total: state.items.length,
    imported,
    duplicate,
    failed: failedCount,
    pending,
    finished: imported + duplicate + failedCount,
  };
}
