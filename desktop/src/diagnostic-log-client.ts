import { randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { diagnosticLogMessage } from './diagnostic-message.ts';
import {
  filteredLogChunks, isRecord, label, logInputFromText, LOG_SOURCES, uuid, validLogRecord,
  type DiagnosticLogInput, type DiagnosticLogRecord,
} from './diagnostic-log.ts';
import type { TelemetryReleaseConfig } from './telemetry-client.ts';
import { TelemetrySettingsStore } from './telemetry-settings.ts';

const SEGMENT_BYTES = 256 * 1024;
const MAX_SPOOL_BYTES = 64 * 1024 * 1024;
const MAX_BATCH_BYTES = 240 * 1024;
const SEGMENT_NAME = /^\d{12}-[0-9a-f-]{36}\.jsonl$/;
const FLUSH_MS = 5_000;

interface SpoolState {
  version: 1;
  consentEpoch: string;
  cursor: { file: string; offset: number } | null;
  droppedRecords: number;
  rejectedRecords: number;
  pendingGap: number;
  lastAcknowledgedAt: string | null;
  lastError: string | null;
}

export interface DiagnosticDeliveryStatus {
  pendingBytes: number;
  pendingFiles: number;
  lastAcknowledgedAt: string | null;
  lastError: string | null;
  droppedRecords: number;
  rejectedRecords: number;
}

interface LogResponse {
  status: number;
  json(): Promise<unknown>;
  headers?: { get(name: string): string | null };
}

type LogFetch = (url: string, options: {
  method: string; headers: Record<string, string>; body: string;
  signal: AbortSignal; redirect: 'error';
}) => Promise<LogResponse>;

export interface DiagnosticLogClientOptions {
  directory: string;
  settings: TelemetrySettingsStore;
  config: TelemetryReleaseConfig | null;
  release: string;
  sessionID: string;
  localLog: (message: string) => void;
  fetch?: LogFetch;
  maxSpoolBytes?: number;
  // Used by the loopback integration harness, never enabled by the desktop.
  allowInsecureLoopback?: boolean;
}

/** An acknowledged disk spool, independent of the small sampled-event queue. */
export class DiagnosticLogClient {
  private readonly options: DiagnosticLogClientOptions;
  private readonly fetch: LogFetch;
  private readonly endpoint: string | null;
  private readonly maxBytes: number;
  private state: SpoolState;
  private sequence = 0;
  private segment = 0;
  private activeFile: string | null = null;
  private timer: NodeJS.Timeout | null = null;
  private flushPromise: Promise<void> | null = null;
  private activeRequest: AbortController | null = null;
  private retryAt = 0;
  private failures = 0;
  private batchLimit = 64;
  private stopped = false;
  private revoked = false;
  private lastHealth = 0;
  private generation = 0;

  constructor(options: DiagnosticLogClientOptions) {
    if (!uuid(options.sessionID)) throw new Error('diagnostic log session must be a UUID');
    this.options = options;
    this.fetch = options.fetch ?? globalThis.fetch;
    this.endpoint = logEndpoint(options.config, options.allowInsecureLoopback === true);
    this.maxBytes = options.maxSpoolBytes ?? MAX_SPOOL_BYTES;
    try { this.state = this.loadState(); }
    catch (error) { this.state = this.emptyState(); this.noteFailure('spool_initialization_failed', error); }
    try { for (const file of this.files()) this.segment = Math.max(this.segment, Number(file.slice(0, 12))); }
    catch (error) { this.noteFailure('spool_read_failed', error); }
  }

  start(): void {
    if (this.timer !== null) return;
    this.stopped = false;
    this.record({ source: 'telemetry', event: 'delivery.started', message: 'Durable diagnostic log delivery started' });
    this.timer = setInterval(() => {
      if (Date.now() - this.lastHealth >= 60_000) {
        this.lastHealth = Date.now();
        const status = this.status();
        this.record({ source: 'telemetry', event: 'delivery.health', level: status.lastError ? 'warn' : 'info',
          message: `pending_bytes=${status.pendingBytes} dropped_records=${status.droppedRecords} rejected_records=${status.rejectedRecords} last_ack=${status.lastAcknowledgedAt ?? 'none'} error=${status.lastError ?? 'none'}` });
      }
      void this.flush();
    }, FLUSH_MS);
    this.timer.unref();
    void this.flush();
  }

  stop(): void {
    this.stopped = true;
    if (this.timer !== null) clearInterval(this.timer);
    this.timer = null;
    this.activeRequest?.abort();
  }

  /** Called on every preference change, including a failed disable operation. */
  resetConsent(enabled: boolean): void {
    this.generation++;
    this.revoked = !enabled;
    this.activeRequest?.abort();
    this.activeFile = null;
    this.retryAt = 0;
    this.failures = 0;
    this.state = this.emptyState();
    try {
      this.clearOwnedFiles();
      this.saveState();
    } catch (error) { this.noteFailure('spool_reset_failed', error); }
  }

  status(): DiagnosticDeliveryStatus {
    let files: string[] = [];
    try { files = this.files(); } catch (error) { this.noteFailure('spool_read_failed', error); }
    let bytes = 0;
    for (const file of files) {
      try { bytes += Math.max(0, fs.statSync(this.filePath(file)).size - (this.state.cursor?.file === file ? this.state.cursor.offset : 0)); }
      catch { /* A file removed during a status read is no longer pending. */ }
    }
    return { pendingBytes: bytes, pendingFiles: files.length,
      lastAcknowledgedAt: this.state.lastAcknowledgedAt, lastError: this.state.lastError,
      droppedRecords: this.state.droppedRecords, rejectedRecords: this.state.rejectedRecords };
  }

  recordText(text: string): void {
    // Delivery diagnostics have their own explicit state records; importing the
    // local uploader's error text here would recursively create upload traffic.
    if (text.startsWith('[telemetry]')) return;
    this.record(logInputFromText(text));
  }

  record(input: DiagnosticLogInput): void {
    if (!this.canRecord()) return;
    try {
      this.ensureEpoch();
      this.publishGap();
      const chunks = filteredLogChunks(input.message);
      for (let index = 0; index < chunks.length; index++) {
        const record = this.build({ ...input, event: index === 0 ? input.event : 'log.continued' }, chunks[index] ?? '');
        this.append(record);
      }
      this.enforceCapacity();
      this.publishGap();
    } catch (error) {
      this.state.pendingGap++;
      this.state.droppedRecords++;
      this.noteFailure('spool_write_failed', error);
    }
  }

  flush(): Promise<void> {
    if (this.flushPromise !== null) return this.flushPromise;
    this.flushPromise = this.flushOne().finally(() => { this.flushPromise = null; });
    return this.flushPromise;
  }

  private canRecord(): boolean {
    return !this.revoked && this.endpoint !== null && this.options.settings.eligible();
  }

  private build(input: Omit<DiagnosticLogInput, 'message'>, message: string): DiagnosticLogRecord {
    const release = this.options.release.split(/[+-]/, 1)[0] ?? '';
    const record: DiagnosticLogRecord = {
      schema_version: 1, id: randomUUID(), support_code: this.options.settings.get().supportCode,
      session_id: this.options.sessionID, sequence: ++this.sequence,
      occurred_at: (input.occurredAt ?? new Date()).toISOString(),
      release: /^\d{1,5}\.\d{1,5}\.\d{1,5}$/.test(release) ? release : '0.0.0',
      source: input.source && LOG_SOURCES.has(input.source) ? input.source : 'studio',
      level: input.level && ['debug','info','warn','error'].includes(input.level) ? input.level : 'info',
      event: label(input.event) ? input.event : 'studio.log', message,
    };
    if (uuid(input.jobID)) record.job_id = input.jobID;
    if (uuid(input.attemptID)) record.attempt_id = input.attemptID;
    if (label(input.operation)) record.operation = input.operation;
    if (input.attempt !== undefined) record.attempt = input.attempt;
    if (input.outcome !== undefined) record.outcome = input.outcome;
    if (input.durationMS !== undefined) record.duration_ms = input.durationMS;
    if (input.exitCode !== undefined) record.exit_code = input.exitCode;
    if (input.lostRecords !== undefined) record.lost_records = input.lostRecords;
    if (!validLogRecord(record)) throw new Error('invalid diagnostic record');
    return record;
  }

  private append(record: DiagnosticLogRecord): void {
    fs.mkdirSync(this.options.directory, { recursive: true, mode: 0o700 });
    const line = `${JSON.stringify(record)}\n`;
    if (this.activeFile === null || fs.statSync(this.filePath(this.activeFile)).size + Buffer.byteLength(line) > SEGMENT_BYTES) {
      this.activeFile = `${String(++this.segment).padStart(12, '0')}-${randomUUID()}.jsonl`;
      fs.writeFileSync(this.filePath(this.activeFile), '', { mode: 0o600, flag: 'wx' });
    }
    fs.appendFileSync(this.filePath(this.activeFile), line, { encoding: 'utf8', mode: 0o600, flush: true });
  }

  private publishGap(): void {
    if (this.state.pendingGap === 0) return;
    const count = this.state.pendingGap;
    this.append(this.build({ source: 'telemetry', event: 'delivery.gap', level: 'error', lostRecords: count },
      `Diagnostic records unavailable before delivery: ${count}. Inspect delivery health; this trace may be incomplete.`));
    this.state.pendingGap = 0;
    this.saveState();
  }

  private enforceCapacity(): void {
    let size = this.status().pendingBytes;
    if (size <= this.maxBytes) return;
    for (const file of this.files()) {
      const data = fs.readFileSync(this.filePath(file));
      const offset = this.state.cursor?.file === file ? this.state.cursor.offset : 0;
      const count = data.subarray(offset).toString('utf8').split('\n').filter(Boolean).length;
      // Persist the gap before eviction so a crash cannot conceal that evidence
      // may have been removed. The 64 MiB cap reserves room for this notice.
      this.state.droppedRecords += count;
      this.state.pendingGap += count;
      if (this.state.cursor?.file === file) this.state.cursor = null;
      this.saveState();
      fs.unlinkSync(this.filePath(file));
      if (this.activeFile === file) this.activeFile = null;
      size -= Math.max(0, data.length - offset);
      if (size <= this.maxBytes - 32 * 1024) break;
    }
  }

  private async flushOne(): Promise<void> {
    if (this.stopped || !this.canRecord() || this.endpoint === null || Date.now() < this.retryAt) return;
    let controller: AbortController | null = null;
    let timeout: NodeJS.Timeout | null = null;
    let generation = this.generation;
    try {
      this.ensureEpoch();
      generation = this.generation;
      this.publishGap();
      const file = this.files()[0];
      if (!file) return;
      const filePath = this.filePath(file);
      if (fs.statSync(filePath).size > SEGMENT_BYTES * 2) {
        this.skipDamaged(file, fs.statSync(filePath).size, 'oversized spool segment');
        return;
      }
      const data = fs.readFileSync(filePath);
      const start = this.state.cursor?.file === file ? this.state.cursor.offset : 0;
      if (start > data.length) { this.skipDamaged(file, data.length, 'spool file shortened after its cursor was saved'); return; }
      const records: DiagnosticLogRecord[] = [];
      let end = start;
      let bytes = 0;
      const remaining = data.subarray(start).toString('utf8');
      let consumed = 0;
      for (const rawLine of remaining.match(/[^\n]*\n|[^\n]+$/g) ?? []) {
        const line = rawLine.endsWith('\n') ? rawLine.slice(0, -1) : rawLine;
        let value: unknown;
        try { value = JSON.parse(line); } catch { value = null; }
        if (!rawLine.endsWith('\n') || !validLogRecord(value)) {
          if (records.length > 0) break;
          this.skipDamaged(file, start + consumed + Buffer.byteLength(rawLine), 'incomplete or invalid persisted log record');
          return;
        }
        value.message = diagnosticLogMessage(value.message);
        const length = Buffer.byteLength(JSON.stringify(value));
        if (records.length > 0 && (records.length >= this.batchLimit || bytes + length > MAX_BATCH_BYTES)) break;
        records.push(value);
        bytes += length + 1;
        consumed += Buffer.byteLength(rawLine);
        end = start + consumed;
      }
      if (records.length === 0) { this.retire(file, end); return; }
      const epoch = this.options.settings.consentEpoch();
      controller = new AbortController();
      this.activeRequest = controller;
      timeout = setTimeout(() => controller?.abort(), 15_000);
      const response = await this.fetch(this.endpoint, {
        method: 'POST', headers: { 'Content-Type': 'application/json', 'X-ClipHub-Ingest-Key': this.options.config?.ingestKey ?? '' },
        body: JSON.stringify({ records }), signal: controller.signal, redirect: 'error',
      });
      if (!this.canRecord() || generation !== this.generation || epoch !== this.options.settings.consentEpoch() || this.stopped) return;
      // Capacity eviction may remove a segment while its request is in flight.
      // Its persisted gap already accounts for the uncertainty; never resurrect
      // a cursor for that segment or overwrite the newer spool state.
      if (!fs.existsSync(filePath)) { this.scheduleFlush(); return; }
      if ([400, 409, 413, 415, 422].includes(response.status)) {
        if (records.length > 1) { this.batchLimit = 1; this.scheduleFlush(); return; }
        const rejected = records[0];
        // Preserve the diagnostic text in a valid replacement record as well
        // as recording the rejection; do not silently erase the first cause.
        if (rejected?.event !== 'delivery.rejected') {
          for (const message of filteredLogChunks(`Collector rejected record ${rejected?.id} with HTTP ${response.status}. Original diagnostic:\n${rejected?.message ?? ''}`)) {
            this.append(this.build({ source: 'telemetry', level: 'error', event: 'delivery.rejected',
              jobID: rejected?.job_id, attemptID: rejected?.attempt_id,
            }, message));
          }
        } else { this.state.pendingGap++; this.state.droppedRecords++; }
        // Persist the replacement before advancing past the rejected evidence.
        this.state.rejectedRecords++;
        this.state.cursor = { file, offset: end };
        this.state.lastError = `collector_rejected_${response.status}`;
        this.saveState();
        this.retire(file, end);
        this.enforceCapacity();
        this.batchLimit = 64;
        this.scheduleFlush();
        return;
      }
      if (response.status !== 202) {
        const retrySeconds = Number(response.headers?.get('retry-after'));
        if (Number.isFinite(retrySeconds) && retrySeconds > 0) this.retryAt = Date.now() + Math.min(retrySeconds, 3600) * 1000;
        throw new Error(`collector_http_${response.status}`);
      }
      const receipt: unknown = await response.json();
      const acceptedIDs = isRecord(receipt) && Array.isArray(receipt.accepted_ids) ? receipt.accepted_ids : null;
      if (acceptedIDs === null || acceptedIDs.length !== records.length ||
        records.some((record, index) => acceptedIDs[index] !== record.id)) throw new Error('collector_receipt_mismatch');
      if (!this.canRecord() || generation !== this.generation || epoch !== this.options.settings.consentEpoch() || this.stopped) return;
      if (!fs.existsSync(filePath)) { this.scheduleFlush(); return; }
      this.state.cursor = { file, offset: end };
      this.state.lastAcknowledgedAt = new Date().toISOString();
      this.state.lastError = null;
      this.saveState();
      this.retire(file, end);
      this.failures = 0;
      this.retryAt = 0;
      this.batchLimit = 64;
      this.scheduleFlush();
    } catch (error) {
      if (!this.canRecord() || generation !== this.generation || this.stopped) return;
      this.failures++;
      this.retryAt = Math.max(this.retryAt, Date.now() + Math.min(30 * 60_000, FLUSH_MS * 2 ** Math.min(this.failures - 1, 9)));
      this.noteFailure('log_upload_deferred', error);
    } finally {
      if (timeout !== null) clearTimeout(timeout);
      if (this.activeRequest === controller) this.activeRequest = null;
    }
  }

  private retire(file: string, offset: number): void {
    if (!fs.existsSync(this.filePath(file)) || fs.statSync(this.filePath(file)).size > offset) return;
    if (this.activeFile === file) this.activeFile = null;
    fs.unlinkSync(this.filePath(file));
    this.state.cursor = null;
    this.saveState();
  }

  private skipDamaged(file: string, offset: number, reason: string): void {
    this.state.cursor = { file, offset };
    this.state.droppedRecords++;
    this.state.pendingGap++;
    this.state.lastError = reason;
    this.saveState();
    this.retire(file, offset);
    this.publishGap();
    this.scheduleFlush();
  }

  private scheduleFlush(): void {
    if (this.stopped || this.files().length === 0) return;
    const timer = setTimeout(() => { void this.flush(); }, 100);
    timer.unref();
  }

  private ensureEpoch(): void {
    if (this.state.consentEpoch === this.options.settings.consentEpoch()) return;
    this.generation++;
    this.activeFile = null;
    this.clearOwnedFiles();
    this.state = this.emptyState();
    this.saveState();
  }

  private emptyState(): SpoolState {
    return { version: 1, consentEpoch: this.options.settings.consentEpoch(), cursor: null,
      droppedRecords: 0, rejectedRecords: 0, pendingGap: 0, lastAcknowledgedAt: null, lastError: null };
  }

  private loadState(): SpoolState {
    try {
      const value: unknown = JSON.parse(fs.readFileSync(path.join(this.options.directory, 'state.json'), 'utf8'));
      if (validSpoolState(value)) return value;
    } catch { /* First run or interrupted/corrupt metadata. Never infer consent. */ }
    const state = this.emptyState();
    const files = this.files();
    if (files.length > 0) {
      // Without the epoch, the old spool is not eligible for upload.
      for (const file of files) {
        try { state.pendingGap += fs.readFileSync(this.filePath(file), 'utf8').split('\n').filter(Boolean).length; } catch { state.pendingGap++; }
      }
      state.droppedRecords = state.pendingGap;
      state.lastError = 'spool_metadata_unavailable';
      this.clearOwnedFiles();
    }
    fs.mkdirSync(this.options.directory, { recursive: true, mode: 0o700 });
    atomicJSON(path.join(this.options.directory, 'state.json'), state);
    return state;
  }

  private files(): string[] {
    try { return fs.readdirSync(this.options.directory).filter((name) => SEGMENT_NAME.test(name)).sort(); }
    catch (error) { if (isRecord(error) && error.code === 'ENOENT') return []; throw error; }
  }

  private filePath(file: string): string {
    if (!SEGMENT_NAME.test(file)) throw new Error('invalid diagnostic spool filename');
    return path.join(this.options.directory, file);
  }

  private clearOwnedFiles(): void {
    for (const file of this.files()) fs.unlinkSync(this.filePath(file));
  }

  private saveState(): void {
    fs.mkdirSync(this.options.directory, { recursive: true, mode: 0o700 });
    atomicJSON(path.join(this.options.directory, 'state.json'), this.state);
  }

  private noteFailure(code: string, error: unknown): void {
    this.state.lastError = `${code}: ${diagnosticLogMessage(error).slice(0, 512)}`;
    try { this.saveState(); } catch { /* Surface disk failure locally even when the spool cannot persist it. */ }
    this.options.localLog(`[telemetry] ${this.state.lastError}\n`);
  }
}

function atomicJSON(file: string, value: unknown): void {
  const temporary = `${file}.${randomUUID()}.tmp`;
  fs.writeFileSync(temporary, `${JSON.stringify(value)}\n`, { mode: 0o600, flush: true });
  try { fs.renameSync(temporary, file); }
  catch (error) { try { fs.unlinkSync(temporary); } catch { /* Preserve the original error. */ } throw error; }
}

function validSpoolState(value: unknown): value is SpoolState {
  if (!isRecord(value) || value.version !== 1 || !uuid(value.consentEpoch)) return false;
  if (value.cursor !== null && (!isRecord(value.cursor) || typeof value.cursor.file !== 'string' || !SEGMENT_NAME.test(value.cursor.file) ||
    typeof value.cursor.offset !== 'number' || !Number.isSafeInteger(value.cursor.offset) || value.cursor.offset < 0)) return false;
  return ['droppedRecords', 'rejectedRecords', 'pendingGap'].every((key) => typeof value[key] === 'number' && Number.isSafeInteger(value[key]) && value[key] >= 0)
    && (value.lastAcknowledgedAt === null || typeof value.lastAcknowledgedAt === 'string')
    && (value.lastError === null || typeof value.lastError === 'string');
}

function logEndpoint(config: TelemetryReleaseConfig | null, loopback: boolean): string | null {
  if (config === null) return null;
  try {
    const url = new URL('/v1/logs', config.endpoint);
    if (url.username || url.password || !config.ingestKey) return null;
    if (url.protocol !== 'https:' && !(loopback && url.protocol === 'http:' && ['127.0.0.1', '[::1]', 'localhost'].includes(url.hostname))) return null;
    return url.href;
  } catch { return null; }
}
