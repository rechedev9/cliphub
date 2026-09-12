import { diagnosticLogMessage } from './diagnostic-message.ts';

export const MAX_LOG_MESSAGE_BYTES = 16 * 1024;
export const LOG_SOURCES = new Set(['studio', 'orchestrator', 'web', 'recorder', 'renderer', 'telemetry']);
const LEVELS = new Set(['debug', 'info', 'warn', 'error']);
const OUTCOMES = new Set(['ok', 'error', 'timeout', 'cancelled', 'interrupted']);

export interface DiagnosticLogInput {
  source?: string;
  level?: string;
  event?: string;
  message: unknown;
  jobID?: string;
  attemptID?: string;
  operation?: string;
  attempt?: number;
  outcome?: string;
  durationMS?: number;
  exitCode?: number;
  lostRecords?: number;
  occurredAt?: Date;
}

export interface DiagnosticLogRecord {
  schema_version: 1;
  id: string;
  support_code: string;
  session_id: string;
  sequence: number;
  occurred_at: string;
  release: string;
  source: string;
  level: string;
  event: string;
  message: string;
  job_id?: string;
  attempt_id?: string;
  operation?: string;
  attempt?: number;
  outcome?: string;
  duration_ms?: number;
  exit_code?: number;
  lost_records?: number;
}

export function uuid(value: unknown): value is string {
  return typeof value === 'string' && /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i.test(value);
}

export function label(value: unknown): value is string {
  return typeof value === 'string' && /^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,95}$/.test(value);
}

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

/** No task payload, paths or arbitrary fields are copied from a trace line. */
export function logInputFromText(text: string): DiagnosticLogInput {
  const prefix = /^\[([^\]]+)\]\s*/.exec(text);
  const source = prefix && LOG_SOURCES.has(prefix[1] ?? '') ? prefix[1] : 'studio';
  const marker = '[cliphub-diagnostic] ';
  const index = text.indexOf(marker);
  if (source === 'orchestrator' && index >= 0) {
    try {
      const entry: unknown = JSON.parse(text.slice(index + marker.length));
      if (isRecord(entry) && label(entry.event) && typeof entry.message === 'string') {
        return {
          source, event: entry.event === 'process.output_gap' ? 'delivery.gap' : entry.event, message: entry.message,
          lostRecords: entry.event === 'process.output_gap' ? 1 : undefined,
          level: typeof entry.level === 'string' && LEVELS.has(entry.level) ? entry.level : 'info',
          jobID: uuid(entry.job_id) ? entry.job_id : undefined,
          attemptID: uuid(entry.attempt_id) ? entry.attempt_id : undefined,
          operation: label(entry.operation) ? entry.operation : undefined,
          attempt: boundedInteger(entry.attempt, 0, 10000),
          outcome: typeof entry.outcome === 'string' && OUTCOMES.has(entry.outcome) ? entry.outcome : undefined,
          durationMS: boundedInteger(entry.duration_ms, 0, 7 * 86_400_000),
          exitCode: boundedInteger(entry.exit_code, -4_294_967_296, 4_294_967_295),
          occurredAt: validDate(entry.time),
        };
      }
    } catch { /* Preserve malformed trace output as technical text, not a fake lifecycle event. */ }
  }
  const job = /\bjob(?:_id)?[=:]([0-9a-f-]{36})\b/i.exec(text)?.[1];
  if (text.includes('[diagnostic-gap]')) return { source, event: 'delivery.gap', level: 'error', message: text, lostRecords: 1 };
  let level = 'info';
  if (/\b(error|failed|fatal|panic|exception)\b/i.test(text)) level = 'error';
  else if (/\b(warn|warning)\b/i.test(text)) level = 'warn';
  return { source, event: 'studio.log', message: text, jobID: uuid(job) ? job : undefined, level };
}

export function filteredLogChunks(message: unknown): string[] {
  // Filter the whole bounded input first, so splitting cannot expose half a
  // credential, path, private-key block or multibyte character.
  const text = diagnosticLogMessage(message);
  if (!text) return [''];
  const chunks: string[] = [];
  let current = '';
  let bytes = 0;
  for (const character of text) {
    const size = Buffer.byteLength(character);
    if (bytes + size > MAX_LOG_MESSAGE_BYTES) { chunks.push(current); current = ''; bytes = 0; }
    current += character;
    bytes += size;
  }
  if (current) chunks.push(current);
  return chunks;
}

export function validLogRecord(value: unknown): value is DiagnosticLogRecord {
  if (!isRecord(value)) return false;
  const required = ['schema_version','id','support_code','session_id','sequence','occurred_at','release','source','level','event','message'];
  const optional = ['job_id','attempt_id','operation','attempt','outcome','duration_ms','exit_code','lost_records'];
  if (required.some((key) => !(key in value)) || Object.keys(value).some((key) => !required.includes(key) && !optional.includes(key))) return false;
  return value.schema_version === 1 && uuid(value.id) && uuid(value.session_id)
    && typeof value.support_code === 'string' && /^CH(?:-[A-F0-9]{4}){5}$/.test(value.support_code)
    && boundedInteger(value.sequence, 1, Number.MAX_SAFE_INTEGER) !== undefined
    && validDate(value.occurred_at) !== undefined
    && typeof value.release === 'string' && /^\d{1,5}\.\d{1,5}\.\d{1,5}$/.test(value.release)
    && typeof value.source === 'string' && LOG_SOURCES.has(value.source)
    && typeof value.level === 'string' && LEVELS.has(value.level)
    && label(value.event) && typeof value.message === 'string' && Buffer.byteLength(value.message) <= MAX_LOG_MESSAGE_BYTES
    && (value.job_id === undefined || uuid(value.job_id)) && (value.attempt_id === undefined || uuid(value.attempt_id))
    && (value.operation === undefined || label(value.operation))
    && (value.attempt === undefined || boundedInteger(value.attempt, 0, 10000) !== undefined)
    && (value.outcome === undefined || typeof value.outcome === 'string' && OUTCOMES.has(value.outcome))
    && (value.duration_ms === undefined || boundedInteger(value.duration_ms, 0, 7 * 86_400_000) !== undefined)
    && (value.exit_code === undefined || boundedInteger(value.exit_code, -4_294_967_296, 4_294_967_295) !== undefined)
    && (value.lost_records === undefined || boundedInteger(value.lost_records, 0, Number.MAX_SAFE_INTEGER) !== undefined);
}

function boundedInteger(value: unknown, minimum: number, maximum: number): number | undefined {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum && value <= maximum ? value : undefined;
}

function validDate(value: unknown): Date | undefined {
  if (typeof value !== 'string') return undefined;
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? undefined : date;
}
