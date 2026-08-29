import { randomUUID } from 'node:crypto';
import * as fs from 'node:fs';
import * as path from 'node:path';
import { TelemetryClient } from './telemetry-client.ts';

const CURSOR_SCHEMA_VERSION = 2;
const POLL_INTERVAL_MS = 10_000;
const MAX_READ_BYTES = 1 << 20;

const ALLOWED_STAGES = new Set([
  'parse', 'record', 'render', 'compose', 'batch', 'http', 'worker',
  'tactical', 'stream_acquire', 'editor', 'short', 'unknown',
]);
const ALLOWED_SPAN_STAGES = new Set(['worker', 'unknown']);
const ALLOWED_CLASSES = new Set([
  'parse:demo', 'scan:roster', 'analyze:anticheat', 'analyze:tactical',
  'record:demo', 'compose:final', 'render:variant', 'render:stream-clip',
  'stream:acquire', 'render:timeline', 'interrupted', 'not_found',
  'auth_required', 'unavailable', 'blocked', 'too_large', 'error',
  'demo_unreadable', 'demo_incompatible', 'map_uncalibrated', 'write_artifact',
  'file_error', 'corrupt', 'target_not_found', 'parse_failed',
  'capture_incompatible', 'unplayable_start', 'record_failed', 'rhythm_failed',
  'render_failed', 'stage_failed', 'write_plan', 'ffmpeg_failed', 'unknown',
]);
const ALLOWED_SPAN_NAMES = new Set([
  'parse:demo', 'scan:roster', 'analyze:anticheat', 'analyze:tactical',
  'record:demo', 'compose:final', 'render:variant', 'render:stream-clip',
  'stream:acquire', 'render:timeline', 'unknown',
]);
const ALLOWED_RESULTS = new Set(['ok', 'error', 'timeout', 'cancelled']);

interface JournalCursor {
  identity: string;
  offset: number;
}

interface JournalCursors {
  schemaVersion: typeof CURSOR_SCHEMA_VERSION;
  errors: JournalCursor;
  spans: JournalCursor;
}

export interface TelemetryJournalOptions {
  client: TelemetryClient;
  errorJournalPath: string;
  spanJournalPath: string;
  cursorPath: string;
  log: (message: string) => void;
}

/** Imports only the fixed safe fields from the local Go observability journals. */
export class TelemetryJournal {
  private readonly client: TelemetryClient;
  private readonly errorJournalPath: string;
  private readonly spanJournalPath: string;
  private readonly cursorPath: string;
  private readonly log: (message: string) => void;
  private timer: NodeJS.Timeout | null = null;
  private cursors: JournalCursors;

  constructor(options: TelemetryJournalOptions) {
    this.client = options.client;
    this.errorJournalPath = options.errorJournalPath;
    this.spanJournalPath = options.spanJournalPath;
    this.cursorPath = options.cursorPath;
    this.log = options.log;
    this.cursors = this.loadCursors();
  }

  start(): void {
    if (this.timer !== null) return;
    this.poll();
    this.timer = setInterval(() => this.poll(), POLL_INTERVAL_MS);
    this.timer.unref();
  }

  stop(): void {
    if (this.timer !== null) {
      clearInterval(this.timer);
      this.timer = null;
    }
  }

  /** Prevents events written before acknowledgement from crossing the boundary. */
  discardPending(): void {
    this.cursors.errors = cursorAtEnd(this.errorJournalPath);
    this.cursors.spans = cursorAtEnd(this.spanJournalPath);
    this.persistCursors();
  }

  poll(): void {
    try {
      const status = this.client.status();
      if (!status.available || !status.enabled || !status.noticeAcknowledged) {
        this.discardPending();
        return;
      }
      this.cursors.errors = readCompleteLines(this.errorJournalPath, this.cursors.errors, (line) => {
        const event = parseErrorLine(line);
        if (event === null) return;
        this.client.recordError({
          component: 'orchestrator',
          name: 'pipeline.error',
          stage: event.stage,
          class: event.class,
          occurredAt: event.time,
        });
      });
      this.cursors.spans = readRotatingLines(this.spanJournalPath, this.cursors.spans, (line) => {
        const span = parseSpanLine(line);
        if (span === null) return;
        this.client.recordSpan({
          component: 'orchestrator',
          name: span.name,
          stage: span.stage,
          outcome: span.result,
          durationMS: span.durationMS,
          occurredAt: span.time,
        });
      });
      this.persistCursors();
    } catch (error) {
      this.log(`[telemetry] local journal cursor deferred: ${String(error)}\n`);
    }
  }

  private loadCursors(): JournalCursors {
    try {
      const value: unknown = JSON.parse(fs.readFileSync(this.cursorPath, 'utf8'));
      if (isRecord(value)
        && value.schemaVersion === CURSOR_SCHEMA_VERSION
        && isJournalCursor(value.errors)
        && isJournalCursor(value.spans)) {
        return {
          schemaVersion: CURSOR_SCHEMA_VERSION,
          errors: value.errors,
          spans: value.spans,
        };
      }
    } catch {
      // First boot starts at the current end, never at historical diagnostics.
    }
    return {
      schemaVersion: CURSOR_SCHEMA_VERSION,
      errors: cursorAtEnd(this.errorJournalPath),
      spans: cursorAtEnd(this.spanJournalPath),
    };
  }

  private persistCursors(): void {
    fs.mkdirSync(path.dirname(this.cursorPath), { recursive: true, mode: 0o700 });
    const temporary = `${this.cursorPath}.${process.pid}.${randomUUID()}.tmp`;
    fs.writeFileSync(temporary, `${JSON.stringify(this.cursors)}\n`, { encoding: 'utf8', mode: 0o600 });
    try {
      fs.renameSync(temporary, this.cursorPath);
    } catch (error) {
      try {
        fs.rmSync(temporary, { force: true });
      } catch {
        // Preserve the original persistence error.
      }
      throw error;
    }
  }
}

function readRotatingLines(
  currentPath: string,
  cursor: JournalCursor,
  handle: (line: string) => void,
): JournalCursor {
  const paths = [4, 3, 2, 1].map((generation) => `${currentPath}.${generation}`).concat(currentPath);
  const snapshots = paths.map(fileSnapshot);
  const cursorIndex = snapshots.findIndex((snapshot) => snapshot?.identity === cursor.identity);
  if (cursorIndex < 0) return readCompleteLines(currentPath, cursor, handle);

  let nextCursor = cursor;
  for (let index = cursorIndex; index < paths.length; index++) {
    const snapshot = snapshots[index];
    if (snapshot === null) continue;
    nextCursor = readCompleteLines(
      paths[index],
      index === cursorIndex ? cursor : { identity: 'missing', offset: 0 },
      handle,
    );
    if (nextCursor.offset < snapshot.size) return nextCursor;
  }
  return nextCursor;
}

function readCompleteLines(filePath: string, cursor: JournalCursor, handle: (line: string) => void): JournalCursor {
  let descriptor: number | null = null;
  try {
    descriptor = fs.openSync(filePath, 'r');
    const stat = fs.fstatSync(descriptor);
    const identity = fileIdentity(stat);
    const requestedOffset = cursor.identity === identity ? cursor.offset : 0;
    const offset = requestedOffset > stat.size ? 0 : requestedOffset;
    const length = Math.min(MAX_READ_BYTES, stat.size - offset);
    if (length <= 0) return { identity, offset };
    const buffer = Buffer.allocUnsafe(length);
    const read = fs.readSync(descriptor, buffer, 0, length, offset);
    const text = buffer.subarray(0, read).toString('utf8');
    const lastNewline = text.lastIndexOf('\n');
    if (lastNewline < 0) {
      // A malformed line larger than the bounded reader must not pin the cursor.
      return read === MAX_READ_BYTES ? { identity, offset: offset + read } : { identity, offset };
    }
    for (const line of text.slice(0, lastNewline).split('\n')) {
      const trimmed = line.trim();
      if (trimmed !== '') handle(trimmed);
    }
    return {
      identity,
      offset: offset + Buffer.byteLength(text.slice(0, lastNewline + 1), 'utf8'),
    };
  } catch (error) {
    if (isMissingFile(error)) return { identity: 'missing', offset: 0 };
    throw error;
  } finally {
    if (descriptor !== null) fs.closeSync(descriptor);
  }
}

function cursorAtEnd(filePath: string): JournalCursor {
  const snapshot = fileSnapshot(filePath);
  return snapshot === null
    ? { identity: 'missing', offset: 0 }
    : { identity: snapshot.identity, offset: snapshot.size };
}

function fileSnapshot(filePath: string): { identity: string; size: number } | null {
  try {
    const stat = fs.statSync(filePath);
    return { identity: fileIdentity(stat), size: stat.size };
  } catch (error) {
    if (isMissingFile(error)) return null;
    throw error;
  }
}

function fileIdentity(stat: fs.Stats): string {
  return `${stat.dev}:${stat.ino}:${stat.birthtimeMs}`;
}

function parseErrorLine(line: string): { time: Date; stage: string; class: string } | null {
  try {
    const value: unknown = JSON.parse(line);
    if (!isRecord(value) || typeof value.stage !== 'string' || typeof value.class !== 'string') return null;
    const time = parsedDate(value.time);
    return time === null ? null : {
      time,
      stage: allowlisted(value.stage, ALLOWED_STAGES),
      class: allowlisted(value.class, ALLOWED_CLASSES),
    };
  } catch {
    return null;
  }
}

function parseSpanLine(line: string): { time: Date; stage: string; name: string; result: string; durationMS: number } | null {
  try {
    const value: unknown = JSON.parse(line);
    if (!isRecord(value)
      || typeof value.stage !== 'string'
      || typeof value.name !== 'string'
      || typeof value.result !== 'string'
      || typeof value.duration_ms !== 'number'
      || !Number.isFinite(value.duration_ms)) return null;
    const time = parsedDate(value.time);
    return time === null ? null : {
      time,
      stage: allowlisted(value.stage, ALLOWED_SPAN_STAGES),
      name: allowlisted(value.name, ALLOWED_SPAN_NAMES),
      result: allowlisted(value.result, ALLOWED_RESULTS),
      durationMS: value.duration_ms,
    };
  } catch {
    return null;
  }
}

function allowlisted(value: string, allowed: ReadonlySet<string>): string {
  return allowed.has(value) ? value : 'unknown';
}

function parsedDate(value: unknown): Date | null {
  if (typeof value !== 'string') return null;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

function isJournalCursor(value: unknown): value is JournalCursor {
  return isRecord(value)
    && typeof value.identity === 'string'
    && typeof value.offset === 'number'
    && Number.isSafeInteger(value.offset)
    && value.offset >= 0;
}

function isMissingFile(error: unknown): boolean {
  return isRecord(error) && error.code === 'ENOENT';
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
