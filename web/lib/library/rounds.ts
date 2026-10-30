import type { TacticalDocument, TacticalRound, TacticalSide } from '../api/tactical.ts';
import { tacticalPerspective } from '../tactical-filter.ts';

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

/** One round of a partida as the Rondas tab lists it, with the tick its link opens. */
export type RoundRow = {
  readonly number: number;
  /** Score before the round, CT first, as the demo counted it. */
  readonly scoreBefore: string;
  readonly winnerSide: TacticalSide | '';
  /** Team key of the team on the winning side, empty for a round with no winner. */
  readonly winnerTeamKey: string;
  readonly round: TacticalRound;
  /** Where the evidence link starts the replay: freeze-time end, when the round goes live. */
  readonly tick: number;
  /** Seconds from freeze-time end to the round end. */
  readonly liveSeconds: number;
};

/** The team key that played a side in a round, mirroring `Document.TeamSide`. */
export function teamKeyOnSide(doc: Pick<TacticalDocument, 'teams'>, round: Pick<TacticalRound, 'half'>, side: TacticalSide): string {
  return doc.teams.find((team) => tacticalPerspective(doc.teams, { team_key: team.key }, round) === side)?.key ?? '';
}

export function roundRows(doc: Pick<TacticalDocument, 'teams' | 'rounds' | 'demo'>): RoundRow[] {
  const rate = doc.demo.tickrate > 0 ? doc.demo.tickrate : 64;
  return doc.rounds.map((round) => ({
    number: round.number,
    scoreBefore: `${round.score_ct_before}-${round.score_t_before}`,
    winnerSide: round.winner,
    winnerTeamKey: round.winner === '' ? '' : teamKeyOnSide(doc, round, round.winner),
    round,
    tick: round.tick_freeze_end,
    liveSeconds: Math.max(0, Math.round((round.tick_end - round.tick_freeze_end) / rate)),
  }));
}
