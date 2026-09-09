import { createHash, randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { TelemetrySettingsStore, type TelemetrySettings } from './telemetry-settings.ts';
import { diagnosticMessage } from './diagnostic-message.ts';

const SCHEMA_VERSION = 1;
const MAX_QUEUE_EVENTS = 200;
const BATCH_EVENTS = 20;
const FLUSH_INTERVAL_MS = 30_000;
const MAX_BACKOFF_MS = 30 * 60_000;
const REQUEST_TIMEOUT_MS = 5_000;
const PERFORMANCE_SAMPLE_RATE = 0.1;
const RELEASE_PATTERN = /^[0-9]{1,5}\.[0-9]{1,5}\.[0-9]{1,5}$/;

export interface TelemetryReleaseConfig {
  endpoint: string;
  ingestKey: string;
}

export interface PublicTelemetryStatus extends TelemetrySettings {
  available: boolean;
  retentionDays: 30;
  performanceSamplePercent: 10;
}

export interface TelemetryErrorInput {
  component: string;
  name: string;
  stage: string;
  class: string;
  occurredAt?: Date;
  message?: unknown;
  jobID?: string;
}

export interface TelemetrySpanInput {
  component: string;
  name: string;
  stage: string;
  outcome: string;
  durationMS: number;
  occurredAt?: Date;
}

interface TelemetryEvent {
  schema_version: typeof SCHEMA_VERSION;
  id: string;
  occurred_at: string;
  kind: 'error' | 'span';
  support_code: string;
  session_id: string;
  release: string;
  component: string;
  name: string;
  stage?: string;
  class?: string;
  message?: string;
  job_id?: string;
  os: string;
  arch: string;
  outcome?: string;
  duration_ms?: number;
}

interface QueueFile {
  schemaVersion: typeof SCHEMA_VERSION;
  consentEpoch: string;
  events: TelemetryEvent[];
}

interface TelemetryFetchResponse {
  ok: boolean;
  status: number;
}

type TelemetryFetch = (
  input: string,
  init: { method: string; headers: Record<string, string>; body: string; signal: AbortSignal },
) => Promise<TelemetryFetchResponse>;

export interface TelemetryClientOptions {
  settings: TelemetrySettingsStore;
  queuePath: string;
  release: string;
  config: TelemetryReleaseConfig | null;
  fetch?: TelemetryFetch;
  log: (message: string) => void;
  performanceSampleRate?: number;
  removeQueue?: (queuePath: string) => void;
}

/** Bounded, offline-tolerant diagnostics queue owned by Electron main. */
export class TelemetryClient {
  private readonly settings: TelemetrySettingsStore;
  private readonly queuePath: string;
  private readonly release: string;
  private readonly config: TelemetryReleaseConfig | null;
  private readonly fetch: TelemetryFetch;
  private readonly log: (message: string) => void;
  private readonly performanceSampleRate: number;
  private readonly removeQueue: (queuePath: string) => void;
  private readonly sessionID = randomUUID();
  private timer: NodeJS.Timeout | null = null;
  private flushPromise: Promise<void> | null = null;
  private failures = 0;
  private nextAttemptAt = 0;
  private activeRequest: AbortController | null = null;
  private batchLimit = BATCH_EVENTS;
  private runtimeRevoked = false;

  constructor(options: TelemetryClientOptions) {
    this.settings = options.settings;
    this.queuePath = options.queuePath;
    this.release = options.release;
    this.config = options.config;
    this.fetch = options.fetch ?? globalThis.fetch;
    this.log = options.log;
    this.performanceSampleRate = options.performanceSampleRate ?? PERFORMANCE_SAMPLE_RATE;
    this.removeQueue = options.removeQueue ?? ((queuePath) => fs.rmSync(queuePath, { force: true }));
  }

  status(): PublicTelemetryStatus {
    return {
      ...this.settings.get(),
      available: this.config !== null,
      retentionDays: 30,
      performanceSamplePercent: 10,
    };
  }

  update(enabled: boolean): PublicTelemetryStatus {
    if (!enabled) {
      this.runtimeRevoked = true;
      this.activeRequest?.abort();
      try {
        this.settings.update(false);
      } finally {
        this.clearQueue();
      }
      return this.status();
    }
    this.settings.update(true);
    this.runtimeRevoked = false;
    this.clearQueue();
    return this.status();
  }

  start(): void {
    if (this.timer !== null) return;
    this.timer = setInterval(() => { void this.flush(); }, FLUSH_INTERVAL_MS);
    this.timer.unref();
    void this.flush();
  }

  stop(): void {
    if (this.timer !== null) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  recordError(input: TelemetryErrorInput): void {
    if (!this.canRecord()) return;
    this.enqueue(this.errorEvent(input));
  }

  recordSpan(input: TelemetrySpanInput): void {
    if (!this.canRecord()) return;
    const event = this.spanEvent(input);
    if (event !== null) this.enqueue(event);
  }

  /** Publish a journal poll once. Errors propagate so its cursors cannot outrun persistence. */
  recordBatch(errors: readonly TelemetryErrorInput[], spans: readonly TelemetrySpanInput[]): void {
    if (!this.canRecord()) return;
    const events: TelemetryEvent[] = [];
    for (const input of errors) events.push(this.errorEvent(input));
    for (const input of spans) {
      const event = this.spanEvent(input);
      if (event !== null) events.push(event);
    }
    this.enqueueBatch(events);
  }

  flush(): Promise<void> {
    if (this.flushPromise !== null) return this.flushPromise;
    this.flushPromise = this.flushNow().finally(() => {
      this.flushPromise = null;
    });
    return this.flushPromise;
  }

  private canRecord(): boolean {
    return !this.runtimeRevoked && this.config !== null && this.settings.eligible();
  }

  private errorEvent(input: TelemetryErrorInput): TelemetryEvent {
    const event = this.baseEvent('error', input.component, input.name, input.occurredAt);
    event.stage = safeLabel(input.stage, 'unknown');
    event.class = safeLabel(input.class, 'unknown');
    const message = diagnosticMessage(input.message);
    if (message) event.message = message;
    if (isUUID(input.jobID)) event.job_id = input.jobID;
    return event;
  }

  private spanEvent(input: TelemetrySpanInput): TelemetryEvent | null {
    const event = this.baseEvent('span', input.component, input.name, input.occurredAt);
    if (!sampled(event.id, this.performanceSampleRate)) return null;
    event.stage = safeLabel(input.stage, 'unknown');
    event.outcome = safeLabel(input.outcome, 'unknown');
    event.duration_ms = Math.max(1, Math.min(86_400_000, Math.round(input.durationMS)));
    return event;
  }

  private baseEvent(
    kind: TelemetryEvent['kind'],
    component: string,
    name: string,
    occurredAt: Date | undefined,
  ): TelemetryEvent {
    return {
      schema_version: SCHEMA_VERSION,
      id: randomUUID(),
      occurred_at: (occurredAt ?? new Date()).toISOString(),
      kind,
      support_code: this.settings.get().supportCode,
      session_id: this.sessionID,
      release: safeRelease(this.release),
      component: safeLabel(component, 'desktop'),
      name: safeLabel(name, 'unknown'),
      os: safeLabel(process.platform, 'unknown'),
      arch: safeLabel(process.arch, 'unknown'),
    };
  }

  private enqueue(event: TelemetryEvent): void {
    try {
      this.enqueueBatch([event]);
    } catch (error) {
      this.log(`[telemetry] local queue unavailable: ${String(error)}\n`);
    }
  }

  private enqueueBatch(events: readonly TelemetryEvent[]): void {
    if (events.length === 0) return;
    const queue = this.readQueue();
    queue.events = queue.events.concat(events).slice(-MAX_QUEUE_EVENTS);
    this.writeQueue(queue);
    if (queue.events.length >= BATCH_EVENTS) void this.flush();
  }

  private async flushNow(): Promise<void> {
    if (!this.canRecord() || this.config === null || Date.now() < this.nextAttemptAt) return;
    const queue = this.readQueue();
    // JSON escaping can make a full diagnostic batch exceed the collector's
    // 64 KiB request limit. Split by serialized bytes before sending it.
    const pending: TelemetryEvent[] = [];
    for (const event of queue.events.slice(0, this.batchLimit)) {
      if (Buffer.byteLength(JSON.stringify({ events: [...pending, event] }), 'utf8') > 60 * 1024) break;
      pending.push(event);
    }
    if (pending.length === 0) return;
    const controller = new AbortController();
    this.activeRequest = controller;
    const timeout = setTimeout(() => controller.abort(), REQUEST_TIMEOUT_MS);
    try {
      const response = await this.fetch(this.config.endpoint, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'X-ClipHub-Ingest-Key': this.config.ingestKey,
        },
        body: JSON.stringify({ events: pending }),
        signal: controller.signal,
      });
      if (isRejectedPayload(response.status)) {
        this.failures = 0;
        this.nextAttemptAt = 0;
        if (pending.length > 1) {
          this.batchLimit = 1;
          this.log('[telemetry] collector rejected a batch; isolating the head event\n');
          this.scheduleFlush();
          return;
        }
        const remaining = this.removeEvents(pending);
        this.batchLimit = BATCH_EVENTS;
        this.log('[telemetry] collector rejected one isolated malformed event; discarded\n');
        if (remaining > 0) this.scheduleFlush();
        return;
      }
      if (!response.ok || response.status !== 202) throw new Error(`collector returned ${response.status}`);
      const remaining = this.removeEvents(pending);
      this.batchLimit = BATCH_EVENTS;
      this.failures = 0;
      this.nextAttemptAt = 0;
      if (remaining > 0) this.scheduleFlush();
    } catch (error) {
      if (!this.settings.eligible()) return;
      this.failures++;
      const backoff = Math.min(MAX_BACKOFF_MS, FLUSH_INTERVAL_MS * (2 ** Math.min(this.failures - 1, 6)));
      this.nextAttemptAt = Date.now() + backoff;
      this.log(`[telemetry] upload deferred: ${error instanceof Error ? error.message : 'request failed'}\n`);
    } finally {
      clearTimeout(timeout);
      if (this.activeRequest === controller) this.activeRequest = null;
    }
  }

  private scheduleFlush(): void {
    const scheduled = setTimeout(() => { void this.flush(); }, 0);
    scheduled.unref();
  }

  private removeEvents(events: TelemetryEvent[]): number {
    const removed = new Set(events.map((event) => event.id));
    const current = this.readQueue();
    current.events = current.events.filter((event) => !removed.has(event.id));
    this.writeQueue(current);
    return current.events.length;
  }

  private readQueue(): QueueFile {
    try {
      const parsed: unknown = JSON.parse(fs.readFileSync(this.queuePath, 'utf8'));
      const queue = parseQueueFile(parsed, this.settings.consentEpoch());
      if (queue !== null) return queue;
    } catch {
      // Missing or corrupt queues start empty; malformed values never upload.
    }
    return { schemaVersion: SCHEMA_VERSION, consentEpoch: this.settings.consentEpoch(), events: [] };
  }

  private writeQueue(queue: QueueFile): void {
    fs.mkdirSync(path.dirname(this.queuePath), { recursive: true, mode: 0o700 });
    const temporary = `${this.queuePath}.${process.pid}.${randomUUID()}.tmp`;
    fs.writeFileSync(temporary, `${JSON.stringify(queue)}\n`, { encoding: 'utf8', mode: 0o600 });
    try {
      fs.renameSync(temporary, this.queuePath);
    } catch (error) {
      try {
        fs.rmSync(temporary, { force: true });
      } catch {
        // Preserve the original persistence error.
      }
      throw error;
    }
  }

  private clearQueue(): void {
    try {
      this.removeQueue(this.queuePath);
    } catch (error) {
      this.log(`[telemetry] could not clear queue: ${String(error)}\n`);
    }
  }
}

function isRejectedPayload(status: number): boolean {
  return status === 400 || status === 413 || status === 415 || status === 422;
}

function parseQueueFile(value: unknown, consentEpoch: string): QueueFile | null {
  if (!isRecord(value)
    || value.schemaVersion !== SCHEMA_VERSION
    || value.consentEpoch !== consentEpoch
    || !Array.isArray(value.events)) return null;
  const keys = Object.keys(value).sort();
  if (keys.join(',') !== 'consentEpoch,events,schemaVersion') return null;
  return {
    schemaVersion: SCHEMA_VERSION,
    consentEpoch,
    events: value.events.filter(isTelemetryEvent).slice(-MAX_QUEUE_EVENTS).map((event) => {
      // Re-sanitize persisted queues too, including after an upgrade.
      if (event.message !== undefined) event.message = diagnosticMessage(event.message);
      return event;
    }),
  };
}

function isTelemetryEvent(value: unknown): value is TelemetryEvent {
  if (!isRecord(value) || (value.kind !== 'error' && value.kind !== 'span')) return false;
  const commonKeys = [
    'schema_version', 'id', 'occurred_at', 'kind', 'support_code', 'session_id',
    'release', 'component', 'name', 'stage', 'os', 'arch',
  ];
  const expectedKeys = value.kind === 'error'
    ? [...commonKeys, 'class']
    : [...commonKeys, 'outcome', 'duration_ms'];
  if (value.kind === 'error' && 'message' in value) expectedKeys.push('message');
  if (value.kind === 'error' && 'job_id' in value) expectedKeys.push('job_id');
  const keys = Object.keys(value).sort();
  expectedKeys.sort();
  if (keys.length !== expectedKeys.length || keys.some((key, index) => key !== expectedKeys[index])) return false;
  const occurredAt = typeof value.occurred_at === 'string' ? new Date(value.occurred_at) : new Date(Number.NaN);
  const now = Date.now();
  const common = value.schema_version === SCHEMA_VERSION
    && isUUID(value.id)
    && !Number.isNaN(occurredAt.getTime())
    && occurredAt.getTime() <= now + 5 * 60_000
    && occurredAt.getTime() >= now - 31 * 24 * 60 * 60_000
    && typeof value.support_code === 'string'
    && /^CH(?:-[A-F0-9]{4}){5}$/.test(value.support_code)
    && isUUID(value.session_id)
    && isRelease(value.release)
    && isLabel(value.component)
    && isLabel(value.name)
    && isLabel(value.stage)
    && isLabel(value.os)
    && isLabel(value.arch);
  if (!common) return false;
  if (value.kind === 'error') return isLabel(value.class)
    && (value.message === undefined || (typeof value.message === 'string' && value.message.length <= 64 * 1024))
    && (value.job_id === undefined || isUUID(value.job_id));
  return isLabel(value.outcome)
    && typeof value.duration_ms === 'number'
    && Number.isSafeInteger(value.duration_ms)
    && value.duration_ms >= 1
    && value.duration_ms <= 86_400_000;
}

function isUUID(value: unknown): value is string {
  return typeof value === 'string'
    && /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

function isLabel(value: unknown): value is string {
  return typeof value === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9_.:/-]{0,95}$/.test(value);
}

function isRelease(value: unknown): value is string {
  return typeof value === 'string' && RELEASE_PATTERN.test(value);
}

function safeLabel(value: string, fallback: string): string {
  const normalized = value.trim().slice(0, 96);
  return /^[a-zA-Z0-9][a-zA-Z0-9_.:/-]{0,95}$/.test(normalized) ? normalized : fallback;
}

function safeRelease(value: string): string {
  const coreVersion = value.trim().split(/[+-]/, 1)[0];
  return RELEASE_PATTERN.test(coreVersion) ? coreVersion : '0.0.0';
}

function sampled(eventID: string, rate: number): boolean {
  if (rate <= 0) return false;
  if (rate >= 1) return true;
  const value = createHash('sha256').update(eventID).digest().readUInt32BE(0);
  return value / 0x1_0000_0000 < rate;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
