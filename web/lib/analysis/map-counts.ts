import type { TeamScoutQuery } from '../api/library.ts';
import type { TeamMapCounts } from '../api/veto.ts';
import { scopeDateRange, type AnalysisScope, type ScopeField } from './scope.ts';
import { teamScopeEntries } from './team-scope.ts';

/** Matches per map name. */
export type MapCounts = Readonly<Record<string, number>>;

/** Why the map selector greys a map out. */
export const NO_MATCHES_TEXT = 'Sin partidas con esta alineación';

/** The scope fields that decide which matches a map has; the round criteria only filter rounds. */
const MATCH_FIELDS: readonly ScopeField[] = ['limit', 'opponentTop', 'whose'];

export type MapCountsQueryOptions = {
  scope: AnalysisScope;
  /** The controls the view reads: the others do not narrow what the view shows. */
  fields: readonly ScopeField[];
  /** SteamID64s of the lineup in scope; empty keeps every match. */
  lineup: readonly string[];
  now: number;
};

/** The query the view's own scope sends, minus the map and side, so counts match what the view shows. */
export function mapCountsQuery({ scope, fields, lineup, now }: MapCountsQueryOptions): TeamScoutQuery {
  const reads = (field: ScopeField): boolean => fields.includes(field);
  const range = reads('period') ? scopeDateRange(scope.period, now) : {};
  return {
    from: range.from,
    to: range.to,
    opponent_ids: reads('opponent') && scope.opponent !== '' ? [scope.opponent] : [],
    lineup: reads('lineup') ? lineup : [],
    criteria: teamScopeEntries(scope, fields.filter((field) => MATCH_FIELDS.includes(field))),
  };
}

export function mapCountsOf(response: TeamMapCounts): MapCounts {
  return Object.fromEntries(response.maps.map((entry) => [entry.map, entry.matches]));
}

/** "3 partidas" beside a map in the selector. */
export function mapCountText(matches: number): string {
  return `${matches} ${matches === 1 ? 'partida' : 'partidas'}`;
}
