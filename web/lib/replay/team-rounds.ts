import { REPLAY_MAX_REFS, type ReplayRef } from '../analysis/replay-link.ts';
import { scopeDateRange, type AnalysisScope } from '../analysis/scope.ts';
import { teamReplayEntries } from '../analysis/team-replay-fields.ts';
import type { TeamScoutQuery } from '../api/library.ts';
import type { TeamRounds } from '../api/team-insights.ts';

/**
 * The rounds query of a team's 2D replay: the scope's map, side, period and rival, the lineup as
 * SteamID64s and the rest of the filters bar as criteria. The same read the team record's Replay tab does.
 */
export function teamRoundsQuery(input: { scope: AnalysisScope; lineup: readonly string[]; now: number }): TeamScoutQuery {
  const { scope, lineup, now } = input;
  const { from, to } = scopeDateRange(scope.period, now);
  return {
    maps: scope.map === '' ? [] : [scope.map],
    side: scope.side,
    from,
    to,
    opponent_ids: scope.opponent === '' ? [] : [scope.opponent],
    lineup,
    criteria: teamReplayEntries(scope),
  };
}

export type TeamRoundRefs = {
  /** The rounds in play order: newest match first, each match's rounds from the first. */
  refs: ReplayRef[];
  /** Every round the scope keeps, before the cap. */
  total: number;
  /** The matches those rounds come from. */
  matchIds: string[];
};

/** The rounds of the answer as replay refs, cut to the longest list the strip can hold. */
export function teamRoundRefs(data: Pick<TeamRounds, 'matches'>, cap: number = REPLAY_MAX_REFS): TeamRoundRefs {
  const all = data.matches.flatMap((match) =>
    [...match.rounds].sort((a, b) => a.round - b.round).map((round) => ({ matchId: match.match_id, round: round.round })),
  );
  const refs = all.slice(0, cap);
  return { refs, total: all.length, matchIds: [...new Set(data.matches.filter((match) => match.rounds.length > 0).map((match) => match.match_id))] };
}
