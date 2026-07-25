import {
  POSITIONS_MAX_SLOTS,
  TACTICAL_BUY_TYPE_ORDER,
  TACTICAL_CT_PATTERN_ORDER,
  TACTICAL_FILTER_PARAMS,
  TACTICAL_OUTCOMES,
  TACTICAL_PHASES,
  TACTICAL_SIDES,
  TACTICAL_SITE_ORDER,
  TACTICAL_T_PATTERN_ORDER,
  tacticalFilterParams,
} from './api/tactical.ts';
import type {
  TacticalBuyType,
  TacticalFilter,
  TacticalOutcome,
  TacticalPhase,
  TacticalRound,
  TacticalSide,
} from './api/tactical.ts';
import { opponentSide } from './tactical-labels.ts';

/**
 * The filter as a linkable URL and as a client-side round predicate.
 *
 * `tacticalplan.FilterFromValues` (Go) is the specification: the same parameter
 * names, the same comma-or-repeat OR semantics, and the same round predicate as
 * `Filter.Match`, so the round list on screen always holds exactly the rounds
 * the aggregate endpoint computed its tendencies from. The one deliberate
 * difference is error handling: a URL is user-editable, so an unknown value is
 * dropped and the rest of the filter still applies, instead of failing the whole
 * request the way the API must.
 *
 * Serialization is not reimplemented here; `tacticalFilterParams` from the API
 * module owns it, and this module is its inverse.
 */

/** The read surface of a query string, satisfied by both `URLSearchParams` and Next's readonly one. */
export type FilterQuery = Pick<URLSearchParams, 'get' | 'getAll'>;

/** Reports whether a filter selects every round (`Filter.Empty`). */
export function isEmptyTacticalFilter(filter: TacticalFilter): boolean {
  return tacticalFilterParams(filter).toString() === '';
}

/** How many independent constraints a filter carries, for a "N filtros" badge. */
export function tacticalFilterCount(filter: TacticalFilter): number {
  const fields = [
    filter.team_key,
    filter.side,
    filter.buys?.length,
    filter.opponent_buys?.length,
    filter.sites?.length,
    filter.outcome,
    filter.t_patterns?.length,
    filter.ct_patterns?.length,
    filter.tags?.length,
    filter.slots?.length,
    filter.round_from,
    filter.round_to,
    filter.phase,
  ];
  return fields.filter((value) => value !== undefined && value !== '' && value !== 0).length;
}

/** Splits one parameter the way Go's `multi` does: repeats and commas both OR. */
function multi(query: FilterQuery, name: string): string[] {
  const out: string[] = [];
  for (const raw of query.getAll(name)) {
    for (const part of raw.split(',')) {
      const trimmed = part.trim();
      if (trimmed !== '') out.push(trimmed);
    }
  }
  return out;
}

function single(query: FilterQuery, name: string): string {
  return (query.get(name) ?? '').trim();
}

function pickKnown<T extends string>(known: readonly T[], raw: readonly string[]): T[] {
  const out: T[] = [];
  for (const item of raw) {
    const lower = item.toLowerCase();
    const match = known.find((candidate) => candidate.toLowerCase() === lower);
    if (match !== undefined && !out.includes(match)) out.push(match);
  }
  return out;
}

function parseSide(raw: string): TacticalSide | undefined {
  const upper = raw.toUpperCase();
  if (upper === TACTICAL_SIDES.ct) return TACTICAL_SIDES.ct;
  if (upper === TACTICAL_SIDES.t) return TACTICAL_SIDES.t;
  return undefined;
}

function parseOutcome(raw: string): TacticalOutcome | undefined {
  const lower = raw.toLowerCase();
  if (lower === TACTICAL_OUTCOMES.win) return TACTICAL_OUTCOMES.win;
  if (lower === TACTICAL_OUTCOMES.loss) return TACTICAL_OUTCOMES.loss;
  return undefined;
}

function parsePhase(raw: string): TacticalPhase | undefined {
  const lower = raw.toLowerCase();
  if (lower === TACTICAL_PHASES.regulation) return TACTICAL_PHASES.regulation;
  if (lower === TACTICAL_PHASES.overtime) return TACTICAL_PHASES.overtime;
  return undefined;
}

/** A round bound is a positive integer; anything else is no bound at all. */
function parseBound(raw: string): number | undefined {
  if (raw === '') return undefined;
  const value = Number(raw);
  if (!Number.isInteger(value) || value < 1) return undefined;
  return value;
}

function parseSlots(raw: readonly string[]): number[] {
  const out: number[] = [];
  for (const item of raw) {
    const value = Number(item);
    if (!Number.isInteger(value) || value < 0 || value >= POSITIONS_MAX_SLOTS) continue;
    if (!out.includes(value)) out.push(value);
  }
  return out.sort((a, b) => a - b);
}

/**
 * Reads a filter out of a query string. Only the parameters
 * `TACTICAL_FILTER_PARAMS` names are consulted, so an unrelated query parameter
 * (a scroll anchor, an analytics tag) never becomes part of the filter.
 */
export function tacticalFilterFromQuery(query: FilterQuery): TacticalFilter {
  const filter: TacticalFilter = {};

  const team = single(query, TACTICAL_FILTER_PARAMS.team);
  if (team !== '') filter.team_key = team;

  const side = parseSide(single(query, TACTICAL_FILTER_PARAMS.side));
  if (side !== undefined) filter.side = side;

  const buys = pickKnown(TACTICAL_BUY_TYPE_ORDER, multi(query, TACTICAL_FILTER_PARAMS.buy));
  if (buys.length > 0) filter.buys = buys;

  const opponentBuys = pickKnown(
    TACTICAL_BUY_TYPE_ORDER,
    multi(query, TACTICAL_FILTER_PARAMS.opponentBuy),
  );
  if (opponentBuys.length > 0) filter.opponent_buys = opponentBuys;

  const sites = pickKnown(TACTICAL_SITE_ORDER, multi(query, TACTICAL_FILTER_PARAMS.site));
  if (sites.length > 0) filter.sites = sites;

  const outcome = parseOutcome(single(query, TACTICAL_FILTER_PARAMS.outcome));
  if (outcome !== undefined) filter.outcome = outcome;

  const tPatterns = pickKnown(
    TACTICAL_T_PATTERN_ORDER,
    multi(query, TACTICAL_FILTER_PARAMS.tPattern),
  );
  if (tPatterns.length > 0) filter.t_patterns = tPatterns;

  const ctPatterns = pickKnown(
    TACTICAL_CT_PATTERN_ORDER,
    multi(query, TACTICAL_FILTER_PARAMS.ctPattern),
  );
  if (ctPatterns.length > 0) filter.ct_patterns = ctPatterns;

  const tags = multi(query, TACTICAL_FILTER_PARAMS.tag);
  if (tags.length > 0) filter.tags = tags;

  const slots = parseSlots(multi(query, TACTICAL_FILTER_PARAMS.slot));
  if (slots.length > 0) filter.slots = slots;

  const from = parseBound(single(query, TACTICAL_FILTER_PARAMS.roundFrom));
  const to = parseBound(single(query, TACTICAL_FILTER_PARAMS.roundTo));
  // An inverted range selects nothing, which reads as a broken page rather than
  // as a filter; the API rejects it outright, so neither bound is applied.
  const orderedRange = from === undefined || to === undefined || from <= to;
  if (orderedRange && from !== undefined) filter.round_from = from;
  if (orderedRange && to !== undefined) filter.round_to = to;

  const phase = parsePhase(single(query, TACTICAL_FILTER_PARAMS.phase));
  if (phase !== undefined) filter.phase = phase;

  return filter;
}

/** The filter as the query string of a linkable, reloadable workspace URL. */
export function tacticalFilterToQuery(filter: TacticalFilter): string {
  return tacticalFilterParams(filter).toString();
}

/** A team's identity, narrowed to what the side lookup needs. */
export type TeamSides = { key: string; start_side: TacticalSide; slots?: number[] };

/**
 * The side a team played in a round, mirroring `Document.TeamSide`: odd halves
 * are the starting side, even halves the swap. Undefined when the team did not
 * play (another map of a series document).
 */
export function teamSideInRound(
  teams: readonly TeamSides[],
  teamKey: string,
  half: number,
): TacticalSide | undefined {
  const team = teams.find((candidate) => candidate.key === teamKey);
  if (team === undefined) return undefined;
  return half % 2 === 1 ? team.start_side : opponentSide(team.start_side);
}

/**
 * The side a filter looks from in a round (`Filter.Perspective`): the team's
 * side when it follows a team, the fixed side when it fixes one, undefined when
 * the filter is side-agnostic.
 */
export function tacticalPerspective(
  teams: readonly TeamSides[],
  filter: TacticalFilter,
  round: Pick<TacticalRound, 'half'>,
): TacticalSide | undefined {
  if (filter.team_key) return teamSideInRound(teams, filter.team_key, round.half);
  return filter.side;
}

function buyForSide(round: TacticalRound, side: TacticalSide): TacticalBuyType {
  return side === TACTICAL_SIDES.ct ? round.economy.ct_buy : round.economy.t_buy;
}

/** Without a perspective a buy matches when either side took it, as Go's `matchBuy` does. */
function matchBuy(
  round: TacticalRound,
  side: TacticalSide | undefined,
  want: readonly TacticalBuyType[],
): boolean {
  if (side === undefined) {
    return want.includes(round.economy.ct_buy) || want.includes(round.economy.t_buy);
  }
  return want.includes(buyForSide(round, side));
}

/**
 * Without a perspective a round is neither a win nor a loss, so asking for one
 * is a filter that cannot be satisfied rather than one that matches everything.
 */
function matchOutcome(
  outcome: TacticalOutcome,
  side: TacticalSide | undefined,
  winner: TacticalSide | '',
): boolean {
  if (side === undefined) return false;
  if (outcome === TACTICAL_OUTCOMES.win) return winner === side;
  return winner !== '' && winner !== side;
}

function matchSlots(
  round: TacticalRound,
  slots: readonly number[],
  side: TacticalSide | undefined,
): boolean {
  return round.players.some(
    (player) => (side === undefined || player.side === side) && slots.includes(player.slot),
  );
}

function matchPhase(round: TacticalRound, phase: TacticalPhase): boolean {
  const overtime = round.overtime ?? 0;
  return phase === TACTICAL_PHASES.overtime ? overtime > 0 : overtime === 0;
}

/**
 * Reports whether a round passes the filter, mirroring `Filter.Match` field for
 * field so the visible round list and the aggregate never disagree.
 */
export function roundMatchesFilter(
  teams: readonly TeamSides[],
  filter: TacticalFilter,
  round: TacticalRound,
): boolean {
  if (filter.round_from !== undefined && round.number < filter.round_from) return false;
  if (filter.round_to !== undefined && round.number > filter.round_to) return false;
  if (filter.phase !== undefined && !matchPhase(round, filter.phase)) return false;

  const side = tacticalPerspective(teams, filter, round);
  // The team did not play this round (a series document, another map).
  if (filter.team_key !== undefined && side === undefined) return false;

  const buys = filter.buys ?? [];
  if (buys.length > 0 && !matchBuy(round, side, buys)) return false;

  const opponentBuys = filter.opponent_buys ?? [];
  if (opponentBuys.length > 0) {
    const opponent = side === undefined ? undefined : opponentSide(side);
    if (!matchBuy(round, opponent, opponentBuys)) return false;
  }

  const sites = filter.sites ?? [];
  if (sites.length > 0 && !sites.includes(round.class.site)) return false;

  if (filter.outcome !== undefined && !matchOutcome(filter.outcome, side, round.winner)) {
    return false;
  }

  const tPatterns = filter.t_patterns ?? [];
  if (tPatterns.length > 0 && !tPatterns.includes(round.class.t_side)) return false;

  const ctPatterns = filter.ct_patterns ?? [];
  if (ctPatterns.length > 0 && !ctPatterns.includes(round.class.ct_side)) return false;

  // Tags compose with AND: every requested tag must be on the round.
  for (const tag of filter.tags ?? []) {
    if (!round.class.tags.includes(tag)) return false;
  }

  const slots = filter.slots ?? [];
  if (slots.length > 0 && !matchSlots(round, slots, side)) return false;

  return true;
}

/** The rounds that pass the filter, in document order (`Filter.Apply`). */
export function filterTacticalRounds(
  teams: readonly TeamSides[],
  filter: TacticalFilter,
  rounds: readonly TacticalRound[],
): TacticalRound[] {
  return rounds.filter((round) => roundMatchesFilter(teams, filter, round));
}
