import {
  CT_ROLES,
  LEADERBOARD_METRICS,
  LEADERBOARD_SOURCES,
  PRO_TOPS,
  T_ROLES,
  type Leaderboard,
  type LeaderboardMatchSource,
  type LeaderboardMetric,
  type LeaderboardQuery,
  type LeaderboardRole,
  type LeaderboardRow,
  type ProTop,
} from '../api/baseline.ts';
import { focusedReplayHref, type ReplayRef } from '../analysis/replay-link.ts';
import { PERIOD_PRESETS, SCOPE_PARAMS, parseAnalysisScope, scopeDateRange, writeAnalysisScope, type AnalysisScope, type ScopeQuery } from '../analysis/scope.ts';

/**
 * The Ranking page: its filters live in the URL (the analysis scope's map,
 * side, period and reference, plus the keys below), so a link restores the view.
 */

/** Columns that sort the table: every leaderboard metric plus the rounds played. */
export type RankingSort = LeaderboardMetric | 'rounds';

export type RankingState = {
  /** Map, side, period and reference (`library` or `pro`) of the analysis scope. */
  readonly scope: AnalysisScope;
  /** A role on the map; it fixes the side. Empty for every player. */
  readonly role: LeaderboardRole | '';
  /** Which kind of demos count; empty for all of them. */
  readonly level: LeaderboardMatchSource | '';
  /** A callout the player holds on the map and side, or `allround`; needs both. Empty for every player. */
  readonly mapRole: string;
  /** Pro data only: the rival is ranked this high or better in HLTV; null for any rival. */
  readonly top: ProTop | null;
  /** Pro data only: the player's own team is ranked this high or better; null for any team. */
  readonly teamTop: ProTop | null;
  /** Pro data only: one event; empty for all of them. */
  readonly event: string;
  readonly minRounds: number;
  readonly sort: RankingSort;
  /** Null leaves the column's natural order. */
  readonly dir: 'asc' | 'desc' | null;
};

export const RANKING_PARAMS = {
  role: 'role',
  mapRole: 'posicion',
  level: 'nivel',
  top: 'rival',
  teamTop: 'equipo',
  event: 'evento',
  minRounds: 'min',
  sort: 'orden',
  dir: 'sentido',
} as const;

/** The ranking reads three months unless the link says another period, `all` included. */
export const DEFAULT_PERIOD = { kind: 'preset', preset: '90d' } as const;
export const DEFAULT_MIN_ROUNDS = 50;
export const MAX_MIN_ROUNDS = 10000;
/** Below this many rounds a figure moves a lot with one match; the table says so. */
export const LOW_SAMPLE_ROUNDS = 50;
export const DEFAULT_SORT: LeaderboardMetric = 'rating';
/** The leaderboard endpoint's cap; the table sorts and filters what it returns. */
export const RANKING_FETCH_LIMIT = 500;

const ROLES: readonly LeaderboardRole[] = [...CT_ROLES, ...T_ROLES];

export function roleSide(role: LeaderboardRole): 'CT' | 'T' {
  return CT_ROLES.some((ct) => ct === role) ? 'CT' : 'T';
}

function parseRole(value: string | null): LeaderboardRole | '' {
  return ROLES.find((role) => role === value) ?? '';
}

function parseLevel(value: string | null): LeaderboardMatchSource | '' {
  return LEADERBOARD_SOURCES.find((source) => source === value) ?? '';
}

function parseTop(value: string | null): ProTop | null {
  return PRO_TOPS.find((top) => String(top) === value) ?? null;
}

function parseSort(value: string | null): RankingSort {
  if (value === 'rounds') return 'rounds';
  return LEADERBOARD_METRICS.find((metric) => metric === value) ?? DEFAULT_SORT;
}

function parseMinRounds(value: string | null): number {
  if (value === null || !/^\d{1,5}$/.test(value)) return DEFAULT_MIN_ROUNDS;
  const n = Number(value);
  return n >= 1 && n <= MAX_MIN_ROUNDS ? n : DEFAULT_MIN_ROUNDS;
}

function parseMapRole(value: string | null): string {
  return value !== null && /^[^\r\n]{1,64}$/.test(value) ? value : '';
}

function parseEvent(value: string | null): string {
  return value !== null && /^[^\r\n]{1,120}$/.test(value) ? value : '';
}

/** A missing period is the default three months; `period=all` is the explicit "every date". */
function rankingScope(query: ScopeQuery): AnalysisScope {
  const scope = parseAnalysisScope(query);
  return scope.period.kind === 'all' && query.get(SCOPE_PARAMS.period) !== 'all' ? { ...scope, period: DEFAULT_PERIOD } : scope;
}

/**
 * Reads the page state. A role needs a map and plays on one side, so the role
 * is dropped without a map and the side follows it.
 */
export function parseRanking(query: ScopeQuery): RankingState {
  const base = rankingScope(query);
  const role = base.map === '' ? '' : parseRole(query.get(RANKING_PARAMS.role));
  const scope = role === '' ? base : { ...base, side: roleSide(role) };
  const pro = scope.reference === 'pro';
  const dir = query.get(RANKING_PARAMS.dir);
  return {
    scope,
    role,
    mapRole: scope.map === '' || scope.side === '' ? '' : parseMapRole(query.get(RANKING_PARAMS.mapRole)),
    level: parseLevel(query.get(RANKING_PARAMS.level)),
    top: pro ? parseTop(query.get(RANKING_PARAMS.top)) : null,
    teamTop: pro ? parseTop(query.get(RANKING_PARAMS.teamTop)) : null,
    event: pro ? parseEvent(query.get(RANKING_PARAMS.event)) : '',
    minRounds: parseMinRounds(query.get(RANKING_PARAMS.minRounds)),
    sort: parseSort(query.get(RANKING_PARAMS.sort)),
    dir: dir === 'asc' || dir === 'desc' ? dir : null,
  };
}

/** The query string with the state written over `current`; defaults are left out and foreign keys stay. */
export function writeRanking(state: RankingState, current: string | URLSearchParams = ''): string {
  const params = new URLSearchParams(writeAnalysisScope(state.scope, current));
  for (const name of Object.values(RANKING_PARAMS)) params.delete(name);
  const { period } = state.scope;
  if (period.kind === 'all') params.set(SCOPE_PARAMS.period, 'all');
  if (period.kind === 'preset' && period.preset !== DEFAULT_PERIOD.preset) params.set(SCOPE_PARAMS.period, period.preset);
  if (period.kind === 'preset' && period.preset === DEFAULT_PERIOD.preset) params.delete(SCOPE_PARAMS.period);
  if (state.role !== '') params.set(RANKING_PARAMS.role, state.role);
  if (state.mapRole !== '') params.set(RANKING_PARAMS.mapRole, state.mapRole);
  if (state.level !== '') params.set(RANKING_PARAMS.level, state.level);
  if (state.top !== null) params.set(RANKING_PARAMS.top, String(state.top));
  if (state.teamTop !== null) params.set(RANKING_PARAMS.teamTop, String(state.teamTop));
  if (state.event !== '') params.set(RANKING_PARAMS.event, state.event);
  if (state.minRounds !== DEFAULT_MIN_ROUNDS) params.set(RANKING_PARAMS.minRounds, String(state.minRounds));
  if (state.sort !== DEFAULT_SORT) params.set(RANKING_PARAMS.sort, state.sort);
  if (state.dir !== null) params.set(RANKING_PARAMS.dir, state.dir);
  return params.toString();
}

/** A change of the state that keeps role, map and side consistent with each other. */
export function patchRanking(state: RankingState, patch: Partial<Omit<RankingState, 'scope'>> & { scope?: Partial<AnalysisScope> }): RankingState {
  const next: RankingState = { ...state, ...patch, scope: { ...state.scope, ...patch.scope } };
  let { role } = next;
  if (next.scope.map === '' || (patch.scope?.side !== undefined && role !== '' && roleSide(role) !== next.scope.side)) role = '';
  const scope = role === '' ? next.scope : { ...next.scope, side: roleSide(role) };
  // A position belongs to one map and side, so a change of either drops it.
  const moved = scope.map !== state.scope.map || scope.side !== state.scope.side;
  const mapRole = moved && patch.mapRole === undefined ? '' : next.mapRole;
  return { ...next, role, mapRole: scope.map === '' || scope.side === '' ? '' : mapRole, scope };
}

/**
 * The pro board always names a period, which is what makes the server rank
 * the pros' maps player by player. "Every date" reads every map, dated or not.
 */
export function proDays(scope: AnalysisScope): number | 'all' {
  if (scope.period.kind !== 'preset') return 'all';
  const { preset } = scope.period;
  return PERIOD_PRESETS.find((entry) => entry.value === preset)?.days ?? 'all';
}

/**
 * The request for the state. The library scope asks for every filter; the pro
 * scope takes side, role, position, period, rival and team level and event, from
 * the community server. The metric is the sort column, so a reference that
 * returns one value per player ranks on what the table sorts by.
 */
export function leaderboardRequest(state: RankingState, now: number): LeaderboardQuery {
  const metric = state.sort === 'rounds' ? DEFAULT_SORT : state.sort;
  const base: LeaderboardQuery = {
    metric,
    scope: state.scope.reference,
    map: state.scope.map === '' ? undefined : state.scope.map,
    min_rounds: state.minRounds,
    limit: RANKING_FETCH_LIMIT,
  };
  if (state.scope.reference === 'pro') {
    return {
      ...base,
      side: state.scope.side === '' ? undefined : state.scope.side,
      role: state.role === '' ? undefined : state.role,
      map_role: state.mapRole === '' ? undefined : state.mapRole,
      days: proDays(state.scope),
      top: state.top ?? undefined,
      team_top: state.teamTop ?? undefined,
      event: state.event === '' ? undefined : state.event,
    };
  }
  const { from, to } = scopeDateRange(state.scope.period, now);
  return {
    ...base,
    side: state.scope.side === '' ? undefined : state.scope.side,
    role: state.role === '' ? undefined : state.role,
    map_role: state.mapRole === '' ? undefined : state.mapRole,
    from,
    to,
    source: state.level === '' ? undefined : state.level,
  };
}

/** The value a row has in a column, or null when the player lacks it. */
export function cellValue(options: { row: LeaderboardRow; sort: RankingSort; boardMetric: LeaderboardMetric }): number | null {
  const { row, sort, boardMetric } = options;
  if (sort === 'rounds') return row.rounds;
  const own = row.metrics?.[sort];
  if (own !== undefined) return own;
  return sort === boardMetric ? row.value : null;
}

/** The direction a column reads first: the best value on top, which for deaths per round is the lowest. */
export function naturalDir(sort: RankingSort): 'asc' | 'desc' {
  return sort === 'dpr' ? 'asc' : 'desc';
}

/** The direction a click on a column asks for: the other one when it is already the sort, else its natural one. */
export function nextDir(state: Pick<RankingState, 'sort' | 'dir'>, sort: RankingSort): 'asc' | 'desc' | null {
  if (state.sort !== sort) return null;
  return (state.dir ?? naturalDir(sort)) === 'asc' ? 'desc' : 'asc';
}

/** Sorts by the column; a player without the value goes last. Ties keep more rounds first. */
export function sortRankingRows(options: {
  rows: readonly LeaderboardRow[];
  state: Pick<RankingState, 'sort' | 'dir'>;
  boardMetric: LeaderboardMetric;
}): LeaderboardRow[] {
  const { rows, state, boardMetric } = options;
  const dir = state.dir ?? naturalDir(state.sort);
  const sign = dir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    const x = cellValue({ row: a, sort: state.sort, boardMetric });
    const y = cellValue({ row: b, sort: state.sort, boardMetric });
    if (x === null && y === null) return 0;
    if (x === null) return 1;
    if (y === null) return -1;
    return x === y ? b.rounds - a.rounds : sign * (x - y);
  });
}

export type PinnedPlayer = { readonly steamid64: string; readonly name: string };

export type RankingEntry = {
  readonly row: LeaderboardRow | null;
  readonly pinned: PinnedPlayer | null;
  /** Place among the ranked rows; null for a pinned player the filter leaves out (below the minimum or not in it). */
  readonly place: number | null;
};

/**
 * Pinned players first, in table order, then the rest. A pinned player the
 * filter leaves out closes the table, so it does not read as the best place.
 */
export function rankingEntries(sorted: readonly LeaderboardRow[], pins: readonly PinnedPlayer[]): RankingEntry[] {
  const pinIds = new Set(pins.map((pin) => pin.steamid64));
  const ranked = new Set(sorted.map((row) => row.steamid64));
  const top: RankingEntry[] = [];
  const rest: RankingEntry[] = [];
  sorted.forEach((row, index) => {
    const pinned = pins.find((entry) => entry.steamid64 === row.steamid64) ?? null;
    (pinned === null ? rest : top).push({ row, pinned, place: index + 1 });
  });
  const missing = pins.filter((pin) => !ranked.has(pin.steamid64) && pinIds.has(pin.steamid64)).map((pin): RankingEntry => ({ row: null, pinned: pin, place: null }));
  return [...top, ...rest, ...missing];
}

const PLAYER_SEARCH_LIMIT = 8;

/** The ranked players whose name holds `term`, not pinned yet; at most a few, in table order. */
export function searchPlayers(options: { rows: readonly LeaderboardRow[]; term: string; pins: readonly PinnedPlayer[] }): LeaderboardRow[] {
  const { rows, term, pins } = options;
  const needle = term.trim().toLowerCase();
  if (needle === '') return [];
  const pinned = new Set(pins.map((pin) => pin.steamid64));
  return rows.filter((row) => !pinned.has(row.steamid64) && row.name.toLowerCase().includes(needle)).slice(0, PLAYER_SEARCH_LIMIT);
}

/** True when the row rests on few rounds, so its figures are a rough read. */
export function lowSample(row: LeaderboardRow): boolean {
  return row.rounds < LOW_SAMPLE_ROUNDS;
}

/** Adds a pin, or removes it when the player is already pinned. */
export function togglePin(pins: readonly PinnedPlayer[], pin: PinnedPlayer): PinnedPlayer[] {
  return pins.some((entry) => entry.steamid64 === pin.steamid64) ? pins.filter((entry) => entry.steamid64 !== pin.steamid64) : [...pins, pin];
}

export const PINS_STORAGE_KEY = 'cliphub.ranking.pinned';

/** Reads the stored pins; anything malformed is dropped. */
export function parsePins(raw: string | null): PinnedPlayer[] {
  if (raw === null) return [];
  try {
    const value: unknown = JSON.parse(raw);
    if (!Array.isArray(value)) return [];
    return value.flatMap((entry): PinnedPlayer[] => {
      if (typeof entry !== 'object' || entry === null) return [];
      const steamid64: unknown = Reflect.get(entry, 'steamid64');
      const name: unknown = Reflect.get(entry, 'name');
      return typeof steamid64 === 'string' && /^[0-9]{1,20}$/.test(steamid64) && typeof name === 'string' ? [{ steamid64, name: name.slice(0, 64) }] : [];
    });
  } catch {
    return [];
  }
}

/** The Replay with the player's rounds of the filter, the player in focus, or null when the row has none. */
export function playerRoundsHref(row: LeaderboardRow, title: string): string | null {
  const examples = row.examples ?? [];
  if (examples.length === 0) return null;
  const refs: ReplayRef[] = examples.map((example) => ({ matchId: example.match_id, round: example.round, tick: example.tick }));
  return focusedReplayHref({ refs, title, steamId: row.steamid64 });
}

/** What the sample behind the table says: "12 partidas · 340 rondas". */
export function boardSampleText(board: Pick<Leaderboard, 'matches' | 'rounds'>): string {
  return `${board.matches} ${board.matches === 1 ? 'partida' : 'partidas'} · ${board.rounds} ${board.rounds === 1 ? 'ronda' : 'rondas'}`;
}
