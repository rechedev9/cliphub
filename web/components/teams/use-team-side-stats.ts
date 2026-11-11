'use client';

import { useEffect, useMemo, useState } from 'react';
import { getTeamSideStats } from '@/lib/api/library';
import type { TeamSideStats } from '@/lib/api/side-stats';
import { scopeDateRange, type AnalysisScope, type ScopeField } from '@/lib/analysis/scope';
import { teamScopeEntries } from '@/lib/analysis/team-scope';
import type { ScopeCounts } from '@/components/analysis/scope-bar';

/** The scope fields the side stats send beyond map, side, period, rival and lineup. */
const SIDE_STATS_CRITERIA: readonly ScopeField[] = ['buy', 'opponentBuy', 'excludeTactics', 'matchTags', 'excludeMatchTags', 'events'];

export type TeamSideStatsQuery = {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  /** "Comparar con" team ids; empty is the whole library. */
  compare: readonly string[];
  onCounts: (counts: ScopeCounts | null) => void;
};

/** The team's side stats for the scope: null while loading, 'failed' when the service did not answer. */
export function useTeamSideStats({ teamId, scope, lineup, compare, onCounts }: TeamSideStatsQuery): TeamSideStats | null | 'failed' {
  const [data, setData] = useState<TeamSideStats | null | 'failed'>(null);
  const [now] = useState(() => Date.now());
  const { map, side, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);
  const criteria = useMemo(() => teamScopeEntries(scope, SIDE_STATS_CRITERIA), [scope]);
  const compareKey = compare.join(',');

  useEffect(() => {
    let active = true;
    setData(null);
    onCounts(null);
    getTeamSideStats(teamId, {
      maps: map === '' ? [] : [map],
      side,
      from,
      to,
      opponent_ids: opponent === '' ? [] : [opponent],
      lineup,
      criteria,
      compare: compareKey === '' ? [] : compareKey.split(','),
    })
      .then((report) => {
        if (!active) return;
        setData(report);
        onCounts({ maps: report.matches, rounds: report.rounds });
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, from, to, opponent, lineup, criteria, compareKey, onCounts]);

  return data;
}
