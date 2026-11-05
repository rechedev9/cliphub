import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

export const PRACTICE_LAUNCH_ENDPOINT = '/api/practice/launch';
export const PRACTICE_MAX_SPOTS = 20;
export const INVALID_PRACTICE_CODE = 'invalid_practice';
export const CS2_NOT_FOUND_CODE = 'cs2_not_found';

const MAP_RE = /^de_[a-z0-9_]{1,32}$/;
const MAX_LABEL_LENGTH = 200;
const GRENADES = ['smoke', 'flash', 'he', 'molotov', 'incendiary', 'decoy'] as const;

export type PracticeGrenade = (typeof GRENADES)[number];

/** One spot: world position (x, y, z) and view angles (pitch, yaw). */
export type PracticeSpot = {
  label: string;
  pos: [number, number, number];
  ang: [number, number];
  grenade?: PracticeGrenade;
};

export type PracticeLaunchInput = {
  map: string;
  spots: PracticeSpot[];
};

/** A spot as CS2 got it: the sanitized label, its console alias and keypad key. */
export type PracticeLaunchedSpot = PracticeSpot & { alias: string; key?: string };

export type PracticeLaunchResult = {
  launched: boolean;
  /** CS2 was already open: the player runs `execCommand` in its console. */
  alreadyRunning: boolean;
  cfgPath: string;
  execCommand: string;
  nextKey: string;
  spots: PracticeLaunchedSpot[];
};

export type PracticeLaunchOutcome =
  | { kind: 'ok'; result: PracticeLaunchResult }
  | { kind: 'invalid'; message: string }
  | { kind: 'cs2_not_found' }
  | { kind: 'offline' }
  | { kind: 'failed'; message: string };

type Parsed<T> = { ok: true; value: T } | { ok: false; error: string };

function field(value: unknown, key: string): unknown {
  return typeof value === 'object' && value !== null ? Reflect.get(value, key) : undefined;
}

function finiteTuple(value: unknown, length: number): number[] | null {
  if (!Array.isArray(value) || value.length !== length) return null;
  const out: number[] = [];
  for (const item of value) {
    if (typeof item !== 'number' || !Number.isFinite(item)) return null;
    out.push(item);
  }
  return out;
}

function isGrenade(value: unknown): value is PracticeGrenade {
  return GRENADES.some((grenade) => grenade === value);
}

function parseSpot(value: unknown): PracticeSpot | null {
  const label = field(value, 'label');
  const pos = finiteTuple(field(value, 'pos'), 3);
  const ang = finiteTuple(field(value, 'ang'), 2);
  const grenade = field(value, 'grenade');
  if (typeof label !== 'string' || label.length > MAX_LABEL_LENGTH || pos === null || ang === null) return null;
  if (grenade !== undefined && !isGrenade(grenade)) return null;
  const spot: PracticeSpot = { label, pos: [pos[0], pos[1], pos[2]], ang: [ang[0], ang[1]] };
  if (grenade !== undefined) spot.grenade = grenade;
  return spot;
}

/** Shape check before the orchestrator, which sanitizes labels and checks ranges. */
export function parsePracticeLaunchInput(value: unknown): Parsed<PracticeLaunchInput> {
  const map = field(value, 'map');
  const spots = field(value, 'spots');
  if (typeof map !== 'string' || !MAP_RE.test(map)) return { ok: false, error: 'map must match de_<name>' };
  if (!Array.isArray(spots) || spots.length === 0 || spots.length > PRACTICE_MAX_SPOTS) {
    return { ok: false, error: `between 1 and ${PRACTICE_MAX_SPOTS} spots` };
  }
  const parsed: PracticeSpot[] = [];
  for (const [index, raw] of spots.entries()) {
    const spot = parseSpot(raw);
    if (spot === null) return { ok: false, error: `spot ${index + 1} is invalid` };
    parsed.push(spot);
  }
  return { ok: true, value: { map, spots: parsed } };
}

type LaunchKeys = { alreadyRunning: string; cfgPath: string; execCommand: string; nextKey: string };

const ORCHESTRATOR_KEYS: LaunchKeys = {
  alreadyRunning: 'already_running',
  cfgPath: 'cfg_path',
  execCommand: 'exec_command',
  nextKey: 'next_key',
};
const PROXY_KEYS: LaunchKeys = {
  alreadyRunning: 'alreadyRunning',
  cfgPath: 'cfgPath',
  execCommand: 'execCommand',
  nextKey: 'nextKey',
};

/** Orchestrator response to the proxy's camelCase whitelist, or null when unusable. */
export function projectPracticeLaunch(body: unknown): PracticeLaunchResult | null {
  return readLaunch(body, ORCHESTRATOR_KEYS);
}

/** The proxy's camelCase response as the client reads it, or null when unusable. */
export function parsePracticeLaunchResult(body: unknown): PracticeLaunchResult | null {
  return readLaunch(body, PROXY_KEYS);
}

function readLaunch(body: unknown, keys: LaunchKeys): PracticeLaunchResult | null {
  const cfgPath = field(body, keys.cfgPath);
  const execCommand = field(body, keys.execCommand);
  const nextKey = field(body, keys.nextKey);
  const rawSpots = field(body, 'spots');
  if (typeof cfgPath !== 'string' || typeof execCommand !== 'string' || typeof nextKey !== 'string') return null;
  if (!Array.isArray(rawSpots)) return null;
  const spots: PracticeLaunchedSpot[] = [];
  for (const raw of rawSpots) {
    const spot = parseSpot(raw);
    const alias = field(raw, 'alias');
    const key = field(raw, 'key');
    if (spot === null || typeof alias !== 'string') return null;
    spots.push(typeof key === 'string' && key !== '' ? { ...spot, alias, key } : { ...spot, alias });
  }
  return {
    launched: field(body, 'launched') === true,
    alreadyRunning: field(body, keys.alreadyRunning) === true,
    cfgPath,
    execCommand,
    nextKey,
    spots,
  };
}

function errorMessage(body: unknown, status: number): string {
  const error = field(body, 'error');
  return typeof error === 'string' && error !== '' ? error : `error del servicio (HTTP ${status})`;
}

/** Writes the practice cfg and opens CS2 on it, or reuses an open CS2. */
export async function launchPractice(input: PracticeLaunchInput): Promise<PracticeLaunchOutcome> {
  let res: Response;
  try {
    res = await fetch(PRACTICE_LAUNCH_ENDPOINT, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(input),
      cache: 'no-store',
    });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body: unknown = await res.json().catch(() => null);
  if (res.ok) {
    const result = parsePracticeLaunchResult(body);
    return result === null ? { kind: 'failed', message: 'respuesta no valida del servicio' } : { kind: 'ok', result };
  }
  const code = field(body, 'code');
  if (res.status === 503 && code === SERVICE_UNAVAILABLE_CODE) return { kind: 'offline' };
  if (code === CS2_NOT_FOUND_CODE) return { kind: 'cs2_not_found' };
  if (res.status === 400) return { kind: 'invalid', message: errorMessage(body, res.status) };
  return { kind: 'failed', message: errorMessage(body, res.status) };
}
