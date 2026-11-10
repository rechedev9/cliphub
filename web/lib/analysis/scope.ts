import type { BaselineScope } from '../api/baseline.ts';
import type { ReportSide } from '../api/scout.ts';
import { TACTICAL_BUY_TYPES, TACTICAL_FILTER_PARAMS, type TacticalBuyType } from '../api/tactical.ts';

/**
 * The analysis scope: the one filter an analysis page (team, player, report)
 * keeps in its URL, so every tab reads the same maps, sides and dates and a
 * reload or a pasted link restores it. The entity comes from the route.
 */

export const PERIOD_PRESETS = [
  { value: '3d', label: 'Últimos 3 días', days: 3 },
  { value: '7d', label: 'Últimos 7 días', days: 7 },
  { value: '14d', label: 'Últimos 14 días', days: 14 },
  { value: '30d', label: 'Últimos 30 días', days: 30 },
  { value: '90d', label: 'Últimos 90 días', days: 90 },
  { value: '180d', label: 'Últimos 180 días', days: 180 },
  { value: '365d', label: 'Último año', days: 365 },
] as const;
export type PeriodPreset = (typeof PERIOD_PRESETS)[number]['value'];

/** Every date, a window ending now, or exact days (`YYYY-MM-DD`, either end may be empty). */
export type ScopePeriod =
  | { readonly kind: 'all' }
  | { readonly kind: 'preset'; readonly preset: PeriodPreset }
  | { readonly kind: 'range'; readonly from: string; readonly to: string };

/**
 * Whose rounds a team view reads: the players of the team's latest match, every
 * player, or a chosen set of SteamID64s (sorted, no repeats).
 */
export type ScopeLineup =
  | { readonly kind: 'latest' }
  | { readonly kind: 'all' }
  | { readonly kind: 'players'; readonly steamIds: readonly string[] };

/** `half` is a semi or a force buy, the one choice that sends two buys to the server. */
export const HALF_BUY = 'half';
export type ScopeBuy = TacticalBuyType | typeof HALF_BUY | '';

/** Rounds in which a side held an AWP, or held none. */
export type ScopePresence = '' | 'with' | 'without';

/** Whose matches a team view reads: the user's own team's or the other teams'. */
export type ScopeWhose = '' | 'own' | 'others';

/** A manual round range, both ends included; null leaves that end open. */
export type ScopeRounds = { readonly from: number | null; readonly to: number | null };

export type AnalysisScope = {
  /** Map name such as `de_mirage`; empty for every map. */
  readonly map: string;
  readonly side: ReportSide;
  readonly period: ScopePeriod;
  /** The most recent N matches; null leaves the page default. */
  readonly limit: number | null;
  readonly buy: ScopeBuy;
  readonly opponentBuy: ScopeBuy;
  /** Rival team id; empty for every rival. */
  readonly opponent: string;
  /** What every comparison on the page reads against: the library mean or the pros'. */
  readonly reference: BaselineScope;
  readonly lineup: ScopeLineup;
  readonly awp: ScopePresence;
  readonly opponentAwp: ScopePresence;
  /** Players alive as `XvY` (own, rival) at some moment of the round; empty for any. */
  readonly advantage: string;
  readonly rounds: ScopeRounds;
  /** Only rivals in this HLTV top; null for every rival. */
  readonly opponentTop: number | null;
  readonly whose: ScopeWhose;
};

export type ScopeField = keyof AnalysisScope;

export const SCOPE_FIELDS: readonly ScopeField[] = [
  'map', 'side', 'period', 'limit', 'buy', 'opponentBuy', 'opponent', 'reference', 'lineup', 'awp', 'opponentAwp', 'advantage',
  'rounds', 'opponentTop', 'whose',
];

/** Query names. Side and buys share the tactical filter's names; the round filters share the server's. */
export const SCOPE_PARAMS = {
  map: 'map',
  side: TACTICAL_FILTER_PARAMS.side,
  period: 'period',
  from: 'from',
  to: 'to',
  limit: 'limit',
  buy: TACTICAL_FILTER_PARAMS.buy,
  opponentBuy: TACTICAL_FILTER_PARAMS.opponentBuy,
  opponent: 'opponent',
  reference: 'ref',
  lineup: 'lineup',
  awp: TACTICAL_FILTER_PARAMS.awp,
  opponentAwp: TACTICAL_FILTER_PARAMS.opponentAwp,
  advantage: TACTICAL_FILTER_PARAMS.advantage,
  roundFrom: TACTICAL_FILTER_PARAMS.roundFrom,
  roundTo: TACTICAL_FILTER_PARAMS.roundTo,
  opponentTop: 'opponent_top',
  whose: 'whose',
} as const;

export const SCOPE_LIMIT_MAX = 50;
/** The server's cap on a lineup (`tacticalplan.MaxLineupPlayers`). */
export const SCOPE_LINEUP_MAX = 50;
/** The `lineup` value of "every player"; a missing one is the latest lineup. */
export const LINEUP_ALL = 'all';
/** The HLTV tops the server takes (`tacticalplan.OpponentTops`). */
export const OPPONENT_TOPS = [5, 15, 30, 100, 200] as const;
/** The most players a side has (`tacticalplan.MaxAdvantagePlayers`). */
export const ADVANTAGE_MAX = 5;
/** Rounds a regulation and overtime match can reach, as the URL bound of a range. */
export const SCOPE_ROUND_MAX = 99;

/** Buys a person can pick: `unknown` is a parser state, not a choice. */
export const SCOPE_BUYS: readonly TacticalBuyType[] = [
  TACTICAL_BUY_TYPES.pistol,
  TACTICAL_BUY_TYPES.eco,
  TACTICAL_BUY_TYPES.semi,
  TACTICAL_BUY_TYPES.force,
  TACTICAL_BUY_TYPES.full,
];

export const DEFAULT_SCOPE: AnalysisScope = {
  map: '',
  side: '',
  period: { kind: 'all' },
  limit: null,
  buy: '',
  opponentBuy: '',
  opponent: '',
  reference: 'library',
  lineup: { kind: 'latest' },
  awp: '',
  opponentAwp: '',
  advantage: '',
  rounds: { from: null, to: null },
  opponentTop: null,
  whose: '',
};

const MAP_RE = /^[a-z0-9_]{1,64}$/;
const STEAM_ID_RE = /^\d{17}$/;
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const DAY_RE = /^\d{4}-\d{2}-\d{2}$/;
const ADVANTAGE_RE = /^([1-5])v([1-5])$/;
const DAY_MS = 24 * 60 * 60 * 1000;

/** The read surface of a query string, satisfied by `URLSearchParams` and Next's readonly one. */
export type ScopeQuery = Pick<URLSearchParams, 'get'>;

function validDay(value: string | null): string {
  if (value === null || !DAY_RE.test(value)) return '';
  const at = Date.parse(`${value}T00:00:00Z`);
  return Number.isNaN(at) || new Date(at).toISOString().slice(0, 10) !== value ? '' : value;
}

function parseSide(value: string | null): ReportSide {
  const side = value?.toUpperCase();
  if (side === 'CT' || side === 'T') return side;
  return '';
}

/** A buy the scope can hold, or empty. */
export function parseScopeBuy(value: string | null): ScopeBuy {
  if (value === HALF_BUY) return HALF_BUY;
  return SCOPE_BUYS.find((buy) => buy === value) ?? '';
}

/** The server buys a scope buy stands for: `half` is semi and force. */
export function scopeBuyTypes(buy: ScopeBuy): TacticalBuyType[] {
  if (buy === '') return [];
  if (buy === HALF_BUY) return [TACTICAL_BUY_TYPES.semi, TACTICAL_BUY_TYPES.force];
  return [buy];
}

function parsePresence(value: string | null): ScopePresence {
  return value === 'with' || value === 'without' ? value : '';
}

function parseWhose(value: string | null): ScopeWhose {
  return value === 'own' || value === 'others' ? value : '';
}

/** `XvY` with each side between 1 and 5 players, or empty. */
export function parseAdvantage(value: string | null): string {
  return value !== null && ADVANTAGE_RE.test(value) ? value : '';
}

function parsePeriod(query: ScopeQuery): ScopePeriod {
  const preset = PERIOD_PRESETS.find((entry) => entry.value === query.get(SCOPE_PARAMS.period));
  if (preset !== undefined) return { kind: 'preset', preset: preset.value };
  const from = validDay(query.get(SCOPE_PARAMS.from));
  const to = validDay(query.get(SCOPE_PARAMS.to));
  if (from === '' && to === '') return { kind: 'all' };
  // A reversed range is read as the person meant it, not as an empty one.
  if (from !== '' && to !== '' && from > to) return { kind: 'range', from: to, to: from };
  return { kind: 'range', from, to };
}

function parseCount(value: string | null, max: number): number | null {
  if (value === null || !/^\d{1,3}$/.test(value)) return null;
  const count = Number(value);
  return count >= 1 && count <= max ? count : null;
}

function parseRounds(query: ScopeQuery): ScopeRounds {
  const from = parseCount(query.get(SCOPE_PARAMS.roundFrom), SCOPE_ROUND_MAX);
  const to = parseCount(query.get(SCOPE_PARAMS.roundTo), SCOPE_ROUND_MAX);
  if (from !== null && to !== null && from > to) return { from: to, to: from };
  return { from, to };
}

function parseOpponentTop(value: string | null): number | null {
  return OPPONENT_TOPS.find((top) => String(top) === value) ?? null;
}

/** A chosen lineup with its ids sorted and without repeats, so one set always writes one URL. */
export function lineupOf(steamIds: readonly string[]): ScopeLineup {
  return { kind: 'players', steamIds: [...new Set(steamIds)].sort() };
}

function parseLineup(value: string | null): ScopeLineup {
  if (value === LINEUP_ALL) return { kind: 'all' };
  if (value === null || value === '') return { kind: 'latest' };
  const ids = value.split(',');
  // One bad id drops the whole set: a partial lineup would silently count other rounds.
  if (ids.length > SCOPE_LINEUP_MAX || !ids.every((steamId) => STEAM_ID_RE.test(steamId))) return { kind: 'latest' };
  return lineupOf(ids);
}

/** Reads a scope from a query string. A value the URL cannot hold is dropped; the rest still applies. */
export function parseAnalysisScope(query: ScopeQuery): AnalysisScope {
  const map = query.get(SCOPE_PARAMS.map) ?? '';
  const opponent = query.get(SCOPE_PARAMS.opponent) ?? '';
  return {
    map: MAP_RE.test(map) ? map : '',
    side: parseSide(query.get(SCOPE_PARAMS.side)),
    period: parsePeriod(query),
    limit: parseCount(query.get(SCOPE_PARAMS.limit), SCOPE_LIMIT_MAX),
    buy: parseScopeBuy(query.get(SCOPE_PARAMS.buy)),
    opponentBuy: parseScopeBuy(query.get(SCOPE_PARAMS.opponentBuy)),
    opponent: UUID_RE.test(opponent) ? opponent.toLowerCase() : '',
    reference: query.get(SCOPE_PARAMS.reference) === 'pro' ? 'pro' : 'library',
    lineup: parseLineup(query.get(SCOPE_PARAMS.lineup)),
    awp: parsePresence(query.get(SCOPE_PARAMS.awp)),
    opponentAwp: parsePresence(query.get(SCOPE_PARAMS.opponentAwp)),
    advantage: parseAdvantage(query.get(SCOPE_PARAMS.advantage)),
    rounds: parseRounds(query),
    opponentTop: parseOpponentTop(query.get(SCOPE_PARAMS.opponentTop)),
    whose: parseWhose(query.get(SCOPE_PARAMS.whose)),
  };
}

function lineupParam(lineup: ScopeLineup): string {
  if (lineup.kind === 'all') return LINEUP_ALL;
  return lineup.kind === 'players' ? lineup.steamIds.join(',') : '';
}

function countParam(value: number | null): string {
  return value === null ? '' : String(value);
}

/**
 * The query string with the scope written over `current`: default values are
 * left out, parameters the scope does not own are kept in place.
 */
export function writeAnalysisScope(scope: AnalysisScope, current: string | URLSearchParams = ''): string {
  const params = new URLSearchParams(current);
  for (const name of Object.values(SCOPE_PARAMS)) params.delete(name);
  const set = (name: string, value: string): void => {
    if (value !== '') params.set(name, value);
  };
  set(SCOPE_PARAMS.map, scope.map);
  set(SCOPE_PARAMS.side, scope.side);
  if (scope.period.kind === 'preset') set(SCOPE_PARAMS.period, scope.period.preset);
  if (scope.period.kind === 'range') {
    set(SCOPE_PARAMS.from, scope.period.from);
    set(SCOPE_PARAMS.to, scope.period.to);
  }
  set(SCOPE_PARAMS.limit, countParam(scope.limit));
  set(SCOPE_PARAMS.buy, scope.buy);
  set(SCOPE_PARAMS.opponentBuy, scope.opponentBuy);
  set(SCOPE_PARAMS.opponent, scope.opponent);
  set(SCOPE_PARAMS.reference, scope.reference === 'pro' ? 'pro' : '');
  set(SCOPE_PARAMS.lineup, lineupParam(scope.lineup));
  set(SCOPE_PARAMS.awp, scope.awp);
  set(SCOPE_PARAMS.opponentAwp, scope.opponentAwp);
  set(SCOPE_PARAMS.advantage, scope.advantage);
  set(SCOPE_PARAMS.roundFrom, countParam(scope.rounds.from));
  set(SCOPE_PARAMS.roundTo, countParam(scope.rounds.to));
  set(SCOPE_PARAMS.opponentTop, countParam(scope.opponentTop));
  set(SCOPE_PARAMS.whose, scope.whose);
  return params.toString();
}

/** True when a field holds anything but its default. */
export function scopeFieldActive(scope: AnalysisScope, field: ScopeField): boolean {
  if (field === 'period') return scope.period.kind !== 'all';
  if (field === 'lineup') return scope.lineup.kind !== 'latest';
  if (field === 'rounds') return scope.rounds.from !== null || scope.rounds.to !== null;
  return scope[field] !== DEFAULT_SCOPE[field];
}

/** The scope with `fields` back at their defaults. */
export function resetScopeFields(scope: AnalysisScope, fields: readonly ScopeField[]): AnalysisScope {
  const reset = new Set(fields);
  const pick = <K extends ScopeField>(field: K): AnalysisScope[K] => (reset.has(field) ? DEFAULT_SCOPE[field] : scope[field]);
  return {
    map: pick('map'),
    side: pick('side'),
    period: pick('period'),
    limit: pick('limit'),
    buy: pick('buy'),
    opponentBuy: pick('opponentBuy'),
    opponent: pick('opponent'),
    reference: pick('reference'),
    lineup: pick('lineup'),
    awp: pick('awp'),
    opponentAwp: pick('opponentAwp'),
    advantage: pick('advantage'),
    rounds: pick('rounds'),
    opponentTop: pick('opponentTop'),
    whose: pick('whose'),
  };
}

/**
 * The period as the RFC 3339 bounds the library API takes. Exact days are
 * whole UTC days, both ends included.
 */
export function scopeDateRange(period: ScopePeriod, now: number): { from?: string; to?: string } {
  if (period.kind === 'preset') {
    const preset = PERIOD_PRESETS.find((entry) => entry.value === period.preset);
    return preset === undefined ? {} : { from: new Date(now - preset.days * DAY_MS).toISOString() };
  }
  if (period.kind === 'range') {
    const range: { from?: string; to?: string } = {};
    if (period.from !== '') range.from = `${period.from}T00:00:00Z`;
    if (period.to !== '') range.to = `${period.to}T23:59:59Z`;
    return range;
  }
  return {};
}

/** True when a played-at date falls inside the period. An unreadable date is kept only by "every date". */
export function inScopePeriod(iso: string, period: ScopePeriod, now: number): boolean {
  if (period.kind === 'all') return true;
  const at = Date.parse(iso);
  if (Number.isNaN(at)) return false;
  const { from, to } = scopeDateRange(period, now);
  if (from !== undefined && at < Date.parse(from)) return false;
  if (to !== undefined && at > Date.parse(to)) return false;
  return true;
}

/** "3 mapas · 72 rondas", the sample size every analysis page shows beside its scope. */
export function scopeCountText(maps: number, rounds: number): string {
  return `${maps} ${maps === 1 ? 'mapa' : 'mapas'} · ${rounds} ${rounds === 1 ? 'ronda' : 'rondas'}`;
}
