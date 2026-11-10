import { TACTICAL_SIDES } from '../api/tactical.ts';
import type { TacticalDocument, TacticalRound } from '../api/tactical.ts';
import { sidesSwapped } from '../tactical-filter.ts';

type ScoredRound = Pick<TacticalRound, 'half' | 'score_ct_before' | 'score_t_before' | 'winner'>;

/** Team names in score order: the team that started CT first, as `teamScoreBefore` orders them. */
export function teamsInScoreOrder(doc: Pick<TacticalDocument, 'teams'>): readonly [string, string] {
  const name = (team: { name: string; key: string } | undefined): string => (team === undefined ? '' : team.name || team.key);
  const starter = doc.teams.find((team) => team.start_side === TACTICAL_SIDES.ct);
  if (starter === undefined) return [name(doc.teams[0]), name(doc.teams[1])];
  return [name(starter), name(doc.teams.find((team) => team !== starter))];
}

function firstTeamWins(doc: Pick<TacticalDocument, 'teams'>, round: ScoredRound): boolean {
  const hasStarter = doc.teams.some((team) => team.start_side === TACTICAL_SIDES.ct);
  // The first team plays CT until the sides swap.
  return (round.winner === TACTICAL_SIDES.ct) !== (hasStarter && sidesSwapped(round.half));
}

/** Score after the round in score order: the score before, plus the winner's point. */
export function scoreAfter(doc: Pick<TacticalDocument, 'teams'>, round: ScoredRound): readonly [number, number] {
  const hasStarter = doc.teams.some((team) => team.start_side === TACTICAL_SIDES.ct);
  const swapped = hasStarter && sidesSwapped(round.half);
  const a = swapped ? round.score_t_before : round.score_ct_before;
  const b = swapped ? round.score_ct_before : round.score_t_before;
  if (round.winner === '') return [a, b];
  return firstTeamWins(doc, round) ? [a + 1, b] : [a, b + 1];
}

/** The winning team's name; empty for a round without a winner. */
export function winnerTeamName(doc: Pick<TacticalDocument, 'teams'>, round: ScoredRound): string {
  if (round.winner === '') return '';
  return teamsInScoreOrder(doc)[firstTeamWins(doc, round) ? 0 : 1];
}
