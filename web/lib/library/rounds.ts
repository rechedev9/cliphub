import type { LibraryTeamKey } from '../api/library.ts';
import type { RoundTimeline, WinPoint } from '../api/metrics.ts';
import type { TacticalBuyType, TacticalDocument, TacticalRound, TacticalSide } from '../api/tactical.ts';
import { strongestKill } from '../metrics/match-view.ts';
import { tacticalPerspective } from '../tactical-filter.ts';
import { opponentSide } from '../tactical-labels.ts';

/** Query keys that open a tactical analysis on one round and tick, the evidence links' target. */
export const TACTICAL_EVIDENCE_QUERY = { round: 'round', tick: 'tick' } as const;
/** Query key of the record tab a link opens, and the tab that holds the replay. */
export const MATCH_TAB_QUERY = 'tab';
export const MATCH_TACTICAL_TAB = 'tactica';

/** The href that opens a partida's replay on one round and tick. */
export function roundEvidenceHref(matchPath: string, round: number, tick: number): string {
  const params = new URLSearchParams({
    [MATCH_TAB_QUERY]: MATCH_TACTICAL_TAB,
    [TACTICAL_EVIDENCE_QUERY.round]: String(round),
    [TACTICAL_EVIDENCE_QUERY.tick]: String(tick),
  });
  return `${matchPath}?${params.toString()}`;
}

/** One value per team of a partida: `ct-start` is the team that started CT, `t-start` the other. */
export type PerTeam<T> = Readonly<Record<LibraryTeamKey, T>>;

/** A winner who started the round below this chance won against the odds. */
export const UPSET_BELOW = 0.35;

/** The kill that moved the win chance most in a round. */
export type RoundSwing = {
  readonly tick: number;
  /** Seconds since the round went live. */
  readonly seconds: number;
  readonly actorSlot?: number;
  readonly targetSlot?: number;
  /** The side the kill favoured, and the chance it gained, 0 to 1. */
  readonly side: TacticalSide;
  readonly gain: number;
};

export type RoundWinChance = {
  /** The winner's chance when the round went live, null for a round with no winner. */
  readonly winnerStart: number | null;
  readonly upset: boolean;
  readonly swing: RoundSwing | null;
};

/** One round of a partida as the Rondas tab lists it, with the tick its link opens. */
export type RoundRow = {
  readonly number: number;
  readonly round: TacticalRound;
  readonly sides: PerTeam<TacticalSide>;
  readonly buys: PerTeam<TacticalBuyType>;
  readonly scoreAfter: PerTeam<number>;
  readonly winnerSide: TacticalSide | '';
  readonly winnerTeamKey: LibraryTeamKey | '';
  /** Where the evidence link starts the replay: freeze-time end, when the round goes live. */
  readonly tick: number;
  /** Seconds from freeze-time end to the round end. */
  readonly liveSeconds: number;
  /** Null when the round has no win chance timeline, such as while the metrics are pending. */
  readonly chance: RoundWinChance | null;
};

/** The team key that played a side in a round, mirroring `Document.TeamSide`. */
export function teamKeyOnSide(doc: Pick<TacticalDocument, 'teams'>, round: Pick<TacticalRound, 'half'>, side: TacticalSide): string {
  return doc.teams.find((team) => tacticalPerspective(doc.teams, { team_key: team.key }, round) === side)?.key ?? '';
}

/** The side the team that started CT played in a round; undefined when the document names no CT starter. */
function ctStarterSide(doc: Pick<TacticalDocument, 'teams'>, round: Pick<TacticalRound, 'half'>): TacticalSide | undefined {
  const ctStarter = doc.teams.find((team) => team.start_side === 'CT');
  if (ctStarter === undefined) return undefined;
  return tacticalPerspective(doc.teams, { team_key: ctStarter.key }, round);
}

/**
 * The score before a round in fixed team order, the team that started CT first
 * (the order of matchScoreLabel). The demo counts it by side, which swaps at the half.
 */
export function teamScoreBefore(
  doc: Pick<TacticalDocument, 'teams'>,
  round: Pick<TacticalRound, 'half' | 'score_ct_before' | 'score_t_before'>,
): readonly [number, number] {
  return ctStarterSide(doc, round) === 'T' ? [round.score_t_before, round.score_ct_before] : [round.score_ct_before, round.score_t_before];
}

/** Empty for a round without a winner, or when the teams are unknown and a side names no team. */
function teamOnSide(sides: PerTeam<TacticalSide> | undefined, side: TacticalSide | ''): LibraryTeamKey | '' {
  if (side === '' || sides === undefined) return '';
  return sides['ct-start'] === side ? 'ct-start' : 't-start';
}

function buyOnSide(round: TacticalRound, side: TacticalSide): TacticalBuyType {
  return side === 'CT' ? round.economy.ct_buy : round.economy.t_buy;
}

export function roundRows(doc: Pick<TacticalDocument, 'teams' | 'rounds' | 'demo'>, timelines: readonly RoundTimeline[] = []): RoundRow[] {
  const rate = doc.demo.tickrate > 0 ? doc.demo.tickrate : 64;
  const byRound = new Map(timelines.map((timeline) => [timeline.round, timeline]));
  return doc.rounds.map((round) => {
    const known = ctStarterSide(doc, round);
    const ctStarter = known ?? 'CT';
    const sides: PerTeam<TacticalSide> = { 'ct-start': ctStarter, 't-start': opponentSide(ctStarter) };
    const winnerTeamKey = teamOnSide(known === undefined ? undefined : sides, round.winner);
    const [a, b] = teamScoreBefore(doc, round);
    const timeline = byRound.get(round.number);
    return {
      number: round.number,
      round,
      sides,
      buys: { 'ct-start': buyOnSide(round, sides['ct-start']), 't-start': buyOnSide(round, sides['t-start']) },
      scoreAfter: { 'ct-start': a + (winnerTeamKey === 'ct-start' ? 1 : 0), 't-start': b + (winnerTeamKey === 't-start' ? 1 : 0) },
      winnerSide: round.winner,
      winnerTeamKey,
      tick: round.tick_freeze_end,
      liveSeconds: Math.max(0, Math.round((round.tick_end - round.tick_freeze_end) / rate)),
      chance: timeline === undefined ? null : roundWinChance(timeline.points, round.winner),
    };
  });
}

/** The winner's chance at the start, whether that was an upset, and the kill that moved it most. */
export function roundWinChance(points: readonly WinPoint[], winner: TacticalSide | ''): RoundWinChance | null {
  const start = points.find((point) => point.kind === 'start') ?? points[0];
  if (start === undefined) return null;
  let winnerStart: number | null = null;
  if (winner !== '') winnerStart = winner === 'CT' ? start.probability_ct : 1 - start.probability_ct;
  return { winnerStart, upset: winnerStart !== null && winnerStart < UPSET_BELOW, swing: biggestKillSwing(points) };
}

/** The kill whose step moved the CT chance most, as the Rondas row shows it. */
function biggestKillSwing(points: readonly WinPoint[]): RoundSwing | null {
  const kill = strongestKill({ round: 0, points });
  if (kill === undefined) return null;
  return {
    tick: kill.tick,
    seconds: kill.seconds,
    actorSlot: kill.point.actor_slot,
    targetSlot: kill.point.target_slot,
    side: kill.delta > 0 ? 'CT' : 'T',
    gain: Math.abs(kill.delta),
  };
}

/** A run of rounds with the sides fixed: a regulation half or an overtime half. */
export type RoundHalf = {
  readonly half: number;
  readonly title: string;
  readonly sides: PerTeam<TacticalSide>;
  /** Rounds each team won in this half. */
  readonly won: PerTeam<number>;
  readonly scoreAfter: PerTeam<number>;
  readonly first: boolean;
  readonly last: boolean;
  readonly rows: readonly RoundRow[];
};

const HALF_NAMES = ['primera mitad', 'segunda mitad'];

function halfTitle(overtime: number, index: number): string {
  const name = HALF_NAMES[index] ?? `mitad ${index + 1}`;
  return overtime > 0 ? `Prórroga ${overtime}, ${name}` : `${name.charAt(0).toUpperCase()}${name.slice(1)}`;
}

/** A half ends where the half counter or the overtime number changes. */
function sameHalf(a: Pick<TacticalRound, 'half' | 'overtime'>, b: Pick<TacticalRound, 'half' | 'overtime'>): boolean {
  return a.half === b.half && (a.overtime ?? 0) === (b.overtime ?? 0);
}

/** Consecutive rounds of the same half and overtime, each with its sides, its own score and the score after it. */
export function roundHalves(rows: readonly RoundRow[]): RoundHalf[] {
  const groups: { head: RoundRow; rows: RoundRow[] }[] = [];
  for (const row of rows) {
    const current = groups.at(-1);
    if (current !== undefined && sameHalf(current.head.round, row.round)) current.rows.push(row);
    else groups.push({ head: row, rows: [row] });
  }
  const halvesPerOvertime = new Map<number, number>();
  return groups.map(({ head, rows: halfRows }, i) => {
    const overtime = head.round.overtime ?? 0;
    const index = halvesPerOvertime.get(overtime) ?? 0;
    halvesPerOvertime.set(overtime, index + 1);
    const tail = halfRows.at(-1) ?? head;
    return {
      half: head.round.half,
      title: halfTitle(overtime, index),
      sides: head.sides,
      won: {
        'ct-start': halfRows.filter((row) => row.winnerTeamKey === 'ct-start').length,
        't-start': halfRows.filter((row) => row.winnerTeamKey === 't-start').length,
      },
      scoreAfter: tail.scoreAfter,
      first: i === 0,
      last: i === groups.length - 1,
      rows: halfRows,
    };
  });
}

/** "6-6" for the first half, "4-7, final 10-13" for the last, the running total in between. */
export function halfScoreText(half: Pick<RoundHalf, 'won' | 'scoreAfter' | 'first' | 'last'>): string {
  const own = `${half.won['ct-start']}-${half.won['t-start']}`;
  if (half.first) return own;
  const total = `${half.scoreAfter['ct-start']}-${half.scoreAfter['t-start']}`;
  return half.last ? `${own}, final ${total}` : `${own}, total ${total}`;
}

export const ROUND_WINNER_FILTERS = ['all', 'ct-start', 't-start'] as const;
export type RoundWinnerFilter = (typeof ROUND_WINNER_FILTERS)[number];

export const ROUND_KIND_FILTERS = ['any', 'pistol', 'eco', 'force', 'upset'] as const;
export type RoundKindFilter = (typeof ROUND_KIND_FILTERS)[number];

export type RoundFilter = { readonly winner: RoundWinnerFilter; readonly kind: RoundKindFilter };

export const EVERY_ROUND: RoundFilter = { winner: 'all', kind: 'any' };

/** Whether a round passes both filters. Eco, force and pistol match when either team took that buy. */
export function keepsRound(filter: RoundFilter, row: RoundRow): boolean {
  if (filter.winner !== 'all' && row.winnerTeamKey !== filter.winner) return false;
  const buys = [row.buys['ct-start'], row.buys['t-start']];
  switch (filter.kind) {
    case 'pistol':
      return buys.includes('pistol') || row.round.class.tags.includes('pistol');
    case 'eco':
      return buys.includes('eco');
    case 'force':
      return buys.includes('force');
    case 'upset':
      return row.chance?.upset === true;
    default:
      return true;
  }
}

/** The halves with only the rounds that pass the filter; a half left empty is dropped. Scores stay those of the whole half. */
export function visibleHalves(halves: readonly RoundHalf[], filter: RoundFilter): RoundHalf[] {
  return halves
    .map((half) => ({ ...half, rows: half.rows.filter((row) => keepsRound(filter, row)) }))
    .filter((half) => half.rows.length > 0);
}

/** How many rounds each winner filter keeps, for the chip counts. */
export function winnerCounts(rows: readonly RoundRow[]): Readonly<Record<RoundWinnerFilter, number>> {
  return {
    all: rows.length,
    'ct-start': rows.filter((row) => row.winnerTeamKey === 'ct-start').length,
    't-start': rows.filter((row) => row.winnerTeamKey === 't-start').length,
  };
}

/** Eco, semi and force: a team that could not or would not buy rifles and full utility. */
export function isWeakBuy(buy: TacticalBuyType): boolean {
  return buy === 'eco' || buy === 'semi' || buy === 'force';
}
