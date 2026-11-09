import {
  CT_ROLES,
  LEADERBOARD_METRICS,
  LEADERBOARD_SOURCES,
  T_ROLES,
  type Leaderboard,
  type LeaderboardMatchSource,
  type LeaderboardMetric,
  type LeaderboardQuery,
  type LeaderboardRole,
  type LeaderboardRow,
} from '../api/baseline.ts';
import { replayHref, type ReplayRef } from '../analysis/replay-link.ts';
import { parseAnalysisScope, scopeDateRange, writeAnalysisScope, type AnalysisScope, type ScopeQuery } from '../analysis/scope.ts';

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
  readonly minRounds: number;
  readonly sort: RankingSort;
  /** Null leaves the column's natural order. */
  readonly dir: 'asc' | 'desc' | null;
};

export const RANKING_PARAMS = { role: 'role', level: 'nivel', minRounds: 'min', sort: 'orden', dir: 'sentido' } as const;

export const RANKING_MIN_ROUNDS = [10, 25, 50, 100, 200] as const;
export const DEFAULT_MIN_ROUNDS = 25;
export const DEFAULT_SORT: LeaderboardMetric = 'rating';
/** The leaderboard endpoint's cap; the table sorts and filters what it returns. */
export const RANKING_FETCH_LIMIT = 500;

const ROLES: readonly LeaderboardRole[] = [...CT_ROLES, ...T_ROLES];

export function roleSide(role: LeaderboardRole): 'CT' | 'T' {
  return (CT_ROLES as readonly string[]).includes(role) ? 'CT' : 'T';
}

function parseRole(value: string | null): LeaderboardRole | '' {
  return ROLES.find((role) => role === value) ?? '';
}

function parseLevel(value: string | null): LeaderboardMatchSource | '' {
  return LEADERBOARD_SOURCES.find((source) => source === value) ?? '';
}

function parseSort(value: string | null): RankingSort {
  if (value === 'rounds') return 'rounds';
  return LEADERBOARD_METRICS.find((metric) => metric === value) ?? DEFAULT_SORT;
}

function parseMinRounds(value: string | null): number {
  const n = Number(value);
  return RANKING_MIN_ROUNDS.find((preset) => preset === n) ?? DEFAULT_MIN_ROUNDS;
}

/**
 * Reads the page state. A role needs a map and plays on one side, so the role
 * is dropped without a map and the side follows it.
 */
export function parseRanking(query: ScopeQuery): RankingState {
  const scope = parseAnalysisScope(query);
  const role = scope.map === '' ? '' : parseRole(query.get(RANKING_PARAMS.role));
  const dir = query.get(RANKING_PARAMS.dir);
  return {
    scope: role === '' ? scope : { ...scope, side: roleSide(role) },
    role,
    level: parseLevel(query.get(RANKING_PARAMS.level)),
    minRounds: parseMinRounds(query.get(RANKING_PARAMS.minRounds)),
    sort: parseSort(query.get(RANKING_PARAMS.sort)),
    dir: dir === 'asc' || dir === 'desc' ? dir : null,
  };
}

/** The query string with the state written over `current`; defaults are left out and foreign keys stay. */
export function writeRanking(state: RankingState, current: string | URLSearchParams = ''): string {
  const params = new URLSearchParams(writeAnalysisScope(state.scope, current));
  for (const name of Object.values(RANKING_PARAMS)) params.delete(name);
  if (state.role !== '') params.set(RANKING_PARAMS.role, state.role);
  if (state.level !== '') params.set(RANKING_PARAMS.level, state.level);
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
  return { ...next, role, scope: role === '' ? next.scope : { ...next.scope, side: roleSide(role) } };
}

/**
 * The request for the state. The library scope asks for every filter; the pro
 * scope takes the map only. The metric is the sort column, so a reference that
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
  if (state.scope.reference === 'pro') return base;
  const { from, to } = scopeDateRange(state.scope.period, now);
  return {
    ...base,
    side: state.scope.side === '' ? undefined : state.scope.side,
    role: state.role === '' ? undefined : state.role,
    from,
    to,
    source: state.level === '' ? undefined : state.level,
  };
}

/** The value a row has in a column, or null when the player lacks it. */
export function cellValue(row: LeaderboardRow, sort: RankingSort, boardMetric: LeaderboardMetric): number | null {
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
export function sortRankingRows(rows: readonly LeaderboardRow[], state: Pick<RankingState, 'sort' | 'dir'>, boardMetric: LeaderboardMetric): LeaderboardRow[] {
  const dir = state.dir ?? naturalDir(state.sort);
  const sign = dir === 'asc' ? 1 : -1;
  return [...rows].sort((a, b) => {
    const x = cellValue(a, state.sort, boardMetric);
    const y = cellValue(b, state.sort, boardMetric);
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
  /** Place among the ranked rows; null for a pinned player the filter leaves out. */
  readonly place: number | null;
};

/** Pinned players first, in table order, then the rest; a pinned player without a row is kept as a gap. */
export function rankingEntries(sorted: readonly LeaderboardRow[], pins: readonly PinnedPlayer[]): RankingEntry[] {
  const pinIds = new Set(pins.map((pin) => pin.steamid64));
  const byId = new Map(sorted.map((row, index) => [row.steamid64, { row, place: index + 1 }]));
  const top: RankingEntry[] = [];
  for (const row of sorted) {
    const pin = pins.find((entry) => entry.steamid64 === row.steamid64);
    if (pin) top.push({ row, pinned: pin, place: byId.get(row.steamid64)?.place ?? null });
  }
  for (const pin of pins) if (!byId.has(pin.steamid64)) top.push({ row: null, pinned: pin, place: null });
  const rest = sorted.flatMap((row, index): RankingEntry[] => (pinIds.has(row.steamid64) ? [] : [{ row, pinned: null, place: index + 1 }]));
  return [...top, ...rest];
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

/** The Replay with the player's newest rounds of the filter, or null when the row has none. */
export function playerRoundsHref(row: LeaderboardRow, title: string): string | null {
  const refs: ReplayRef[] = (row.examples ?? []).map((example) => ({ matchId: example.match_id, round: example.round, tick: example.tick }));
  return refs.length === 0 ? null : replayHref(refs, title);
}

/** What the sample behind the table says: "12 partidas · 340 rondas". */
export function boardSampleText(board: Pick<Leaderboard, 'matches' | 'rounds'>): string {
  return `${board.matches} ${board.matches === 1 ? 'partida' : 'partidas'} · ${board.rounds} ${board.rounds === 1 ? 'ronda' : 'rondas'}`;
}
