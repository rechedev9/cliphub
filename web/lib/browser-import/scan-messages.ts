export type ScanResult =
  | {
      ok: true;
      sha256: string;
      map: string;
      rounds: number;
      cs2Build: number;
      document: string;
      positions: Uint8Array;
    }
  | { ok: false; error: string };

export interface ScanProgressMessage {
  id: string;
  type: 'progress';
  read: number;
  total: number;
}

export interface ScanResultMessage {
  id: string;
  type: 'result';
  result: ScanResult;
}

export function parseProgressMessage(value: unknown): ScanProgressMessage | undefined {
  if (!isObject(value)) return undefined;
  if (value.type !== 'progress') return undefined;
  if (!isString(value.id)) return undefined;
  if (!isNumber(value.read) || !isNumber(value.total)) return undefined;
  return { id: value.id, type: 'progress', read: value.read, total: value.total };
}

export function parseResultMessage(value: unknown): ScanResultMessage | undefined {
  if (!isObject(value)) return undefined;
  if (value.type !== 'result') return undefined;
  if (!isString(value.id)) return undefined;
  const result = parseScanResult(value.result);
  if (result === undefined) return undefined;
  return { id: value.id, type: 'result', result };
}

function parseScanResult(value: unknown): ScanResult | undefined {
  if (!isObject(value)) return undefined;
  if (value.ok === true) {
    if (!isString(value.sha256)) return undefined;
    if (!isString(value.map)) return undefined;
    if (!isNumber(value.rounds)) return undefined;
    if (!isNumber(value.cs2Build)) return undefined;
    if (!isString(value.document)) return undefined;
    if (!(value.positions instanceof Uint8Array)) return undefined;
    return {
      ok: true,
      sha256: value.sha256,
      map: value.map,
      rounds: value.rounds,
      cs2Build: value.cs2Build,
      document: value.document,
      positions: value.positions,
    };
  }
  if (value.ok === false) {
    if (!isString(value.error)) return undefined;
    return { ok: false, error: value.error };
  }
  return undefined;
}

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isString(value: unknown): value is string {
  return typeof value === 'string';
}

function isNumber(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value);
}
