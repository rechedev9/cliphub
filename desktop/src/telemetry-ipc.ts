export const STUDIO_TELEMETRY_EVENT_CHANNEL = 'cliphub:telemetry-event';

const ERROR_NAMES = new Set(['route.error', 'global.error']);
const SPAN_NAMES = new Set(['navigation.dom_content_loaded', 'navigation.load']);

export type TelemetryEventRequest =
  | { kind: 'error'; name: string }
  | { kind: 'span'; name: string; durationMS: number };

/** Accepts only fixed renderer event codes, never labels, attributes, or text. */
export function parseTelemetryEventRequest(value: unknown): TelemetryEventRequest {
  if (!isRecord(value) || typeof value.kind !== 'string') throw new Error('invalid telemetry event');
  if (value.kind === 'error') {
    requireExactKeys(value, ['kind', 'name']);
    if (typeof value.name !== 'string' || !ERROR_NAMES.has(value.name)) throw new Error('invalid telemetry event');
    return { kind: 'error', name: value.name };
  }
  if (value.kind === 'span') {
    requireExactKeys(value, ['kind', 'name', 'durationMS']);
    if (typeof value.name !== 'string' || !SPAN_NAMES.has(value.name)) throw new Error('invalid telemetry event');
    const durationMS = value.durationMS;
    if (typeof durationMS !== 'number' || !Number.isFinite(durationMS) || durationMS < 0 || durationMS > 86_400_000) {
      throw new Error('invalid telemetry event');
    }
    return { kind: 'span', name: value.name, durationMS };
  }
  throw new Error('invalid telemetry event');
}

function requireExactKeys(value: Record<string, unknown>, expected: string[]): void {
  const keys = Object.keys(value).sort();
  const wanted = [...expected].sort();
  if (keys.length !== wanted.length || keys.some((key, index) => key !== wanted[index])) {
    throw new Error('invalid telemetry event');
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
