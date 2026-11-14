'use client';

import { useMemo } from 'react';
import { latestLineupMissesMap, lineupForMap, lineupSteamIds, type MapLineup } from '@/lib/analysis/lineup';
import type { AnalysisScope, ScopeField } from '@/lib/analysis/scope';
import type { TeamLineups } from '@/lib/api/team-insights';
import { useTeamMapCounts } from './use-team-map-counts';

export type MapLineupOptions = {
  teamId: string;
  scope: AnalysisScope;
  fields: readonly ScopeField[];
  lineups: TeamLineups | null;
};

/** The lineup a one-map team view sends, with its map counts: every player when the default lineup never played the map. */
export function useMapLineup({ teamId, scope, fields, lineups }: MapLineupOptions): MapLineup {
  const chosenKey = lineupSteamIds(scope.lineup, lineups).join(',');
  const chosen = useMemo(() => (chosenKey === '' ? [] : chosenKey.split(',')), [chosenKey]);
  const latestCounts = useTeamMapCounts({ teamId, scope, fields, lineup: chosen, enabled: true });
  const misses = fields.includes('lineup') && latestLineupMissesMap({ lineup: scope.lineup, lineups, map: scope.map, mapCounts: latestCounts });
  const everyoneCounts = useTeamMapCounts({ teamId, scope, fields, lineup: [], enabled: misses });
  return useMemo(
    () => lineupForMap({ chosen, map: scope.map, latestCounts, everyoneCounts, misses }),
    [chosen, scope.map, latestCounts, everyoneCounts, misses],
  );
}
