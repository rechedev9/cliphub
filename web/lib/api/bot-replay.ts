import { SERVICE_UNAVAILABLE_CODE } from './types.ts';
import type { TacticalDocument, TacticalRound } from './tactical.ts';

/** The body fields and response keys the library proxy lets through for the bot replay route. */
export const BOT_REPLAY_FIELDS = ['side', 'replace_steamid'] as const;
export const BOT_REPLAY_KEYS = [
  'launched', 'already_running', 'cfg_path', 'exec_command', 'map', 'round', 'side', 'bot_side',
  'keys', 'bots', 'starts', 'current', 'moments', 'lineups', 'limits',
] as const;

const CS2_NOT_FOUND_CODE = 'cs2_not_found';
const NO_POSITIONS_CODE = 'no_positions';

export type BotReplaySide = 'T' | 'CT';

export type BotReplayInput = {
  matchId: string;
  round: number;
  side: BotReplaySide;
  /** A player of `side` whose spawn and positions the user takes; empty takes the first. */
  replaceSteamId: string;
};

export type BotReplayMoment = { seconds: number; label: string; notes: string[] };
export type BotReplayLineup = { seconds: number; kind: string; player: string; key: string };
export type BotReplayPlayer = { name: string; steamid64: string };

export type BotReplayResult = {
  launched: boolean;
  alreadyRunning: boolean;
  execCommand: string;
  map: string;
  round: number;
  botSide: string;
  keys: { restart: string; next: string; start: string };
  bots: BotReplayPlayer[];
  startsAs: string;
  moments: BotReplayMoment[];
  lineups: BotReplayLineup[];
  limits: string[];
};

export type BotReplayOutcome =
  | { kind: 'ok'; result: BotReplayResult }
  | { kind: 'cs2_not_found' }
  | { kind: 'no_positions' }
  | { kind: 'offline' }
  | { kind: 'failed'; message: string };

function field(value: unknown, key: string): unknown {
  return typeof value === 'object' && value !== null ? Reflect.get(value, key) : undefined;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function count(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : 0;
}

function list<T>(value: unknown, read: (item: unknown) => T): T[] {
  return Array.isArray(value) ? value.map(read) : [];
}

function strings(value: unknown): string[] {
  return list(value, text).filter((item) => item !== '');
}

const GRENADE_LABELS: Readonly<Record<string, string>> = { smoke: 'Humo', flash: 'Flash', he: 'HE', molotov: 'Molotov', incendiary: 'Incendiaria' };

/** The Spanish name of a lineup's grenade kind. */
export function botReplayGrenadeLabel(kind: string): string {
  return GRENADE_LABELS[kind] ?? kind;
}

/** The players of one side in a round, by slot, as the user can start as them. */
export function sidePlayers({
  doc,
  round,
  side,
}: {
  doc: Pick<TacticalDocument, 'players'>;
  round: Pick<TacticalRound, 'players'>;
  side: BotReplaySide;
}): BotReplayPlayer[] {
  return round.players
    .filter((player) => player.side === side)
    .toSorted((a, b) => a.slot - b.slot)
    .flatMap((player) => {
      const identity = doc.players.find((entry) => entry.slot === player.slot);
      return identity === undefined ? [] : [{ name: identity.name, steamid64: identity.steamid64 }];
    });
}

/** The orchestrator's bot replay answer as the dialog reads it, or null when unusable. */
export function parseBotReplayResult(body: unknown): BotReplayResult | null {
  const keys = field(body, 'keys');
  const execCommand = field(body, 'exec_command');
  const moments = field(body, 'moments');
  if (typeof execCommand !== 'string' || !Array.isArray(moments) || typeof keys !== 'object' || keys === null) return null;
  const starts = list(field(body, 'starts'), (item) => text(field(item, 'name')));
  const current = count(field(body, 'current'));
  return {
    launched: field(body, 'launched') === true,
    alreadyRunning: field(body, 'already_running') === true,
    execCommand,
    map: text(field(body, 'map')),
    round: count(field(body, 'round')),
    botSide: text(field(body, 'bot_side')),
    keys: { restart: text(field(keys, 'restart')), next: text(field(keys, 'next')), start: text(field(keys, 'start')) },
    bots: list(field(body, 'bots'), (item) => ({ name: text(field(item, 'name')), steamid64: text(field(item, 'steamid64')) })),
    startsAs: starts[current] ?? '',
    moments: moments.map((item) => ({ seconds: count(field(item, 'seconds')), label: text(field(item, 'label')), notes: strings(field(item, 'notes')) })),
    lineups: list(field(body, 'lineups'), (item) => ({
      seconds: count(field(item, 'seconds')),
      kind: text(field(item, 'kind')),
      player: text(field(item, 'player')),
      key: text(field(item, 'key')),
    })),
    limits: strings(field(body, 'limits')),
  };
}

/** Writes the bot replay cfg of a library round and opens CS2 on it. */
export async function launchBotReplay(input: BotReplayInput): Promise<BotReplayOutcome> {
  const url = `/api/library/matches/${encodeURIComponent(input.matchId)}/tactical/rounds/${input.round}/bot-replay`;
  const payload: Record<string, string> = { side: input.side };
  if (input.replaceSteamId !== '') payload.replace_steamid = input.replaceSteamId;
  let res: Response;
  try {
    res = await fetch(url, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify(payload),
      cache: 'no-store',
    });
  } catch (err) {
    return { kind: 'failed', message: `no se pudo contactar con el servicio local: ${String(err)}` };
  }
  const body: unknown = await res.json().catch(() => null);
  if (res.ok) {
    const result = parseBotReplayResult(body);
    return result === null ? { kind: 'failed', message: 'respuesta no valida del servicio' } : { kind: 'ok', result };
  }
  const code = field(body, 'code');
  if (res.status === 503 && code === SERVICE_UNAVAILABLE_CODE) return { kind: 'offline' };
  if (code === CS2_NOT_FOUND_CODE) return { kind: 'cs2_not_found' };
  if (code === NO_POSITIONS_CODE) return { kind: 'no_positions' };
  const error = field(body, 'error');
  return { kind: 'failed', message: typeof error === 'string' && error !== '' ? error : `error del servicio (HTTP ${res.status})` };
}
