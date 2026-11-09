'use client';

import { useCallback, useState, type ReactNode } from 'react';
import type { SpatialResponse } from '@/lib/api/spatial';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

/** A team's rounds on the 2D radar, one map at a time, read through the page's analysis scope. */
export function TeamSpatial({
  teamId,
  scope,
  lineup,
  view,
  onMapChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  /** The form the team's Replay is on: a heatmap or the superposed paths. */
  view: 'heatmap' | 'overlay';
  onMapChange: (map: string) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);
  const onLoaded = useCallback(
    (response: SpatialResponse) => {
      const rounds = response.heatmap?.rounds ?? response.overlay?.total ?? 0;
      onCounts({ maps: response.matches.length, rounds });
    },
    [onCounts],
  );

  return (
    <SpatialWorkspace
      side={scope.side}
      view={view}
      hideViewSwitch
      onMapChange={onMapChange}
      onLoaded={onLoaded}
      scope={{
        kind: 'team',
        teamId,
        query: {
          map: scope.map === '' ? undefined : scope.map,
          side: scope.side,
          opponent_ids: scope.opponent === '' ? [] : [scope.opponent],
          from,
          to,
          lineup,
          buys: scope.buy === '' ? [] : [scope.buy],
          opponent_buys: scope.opponentBuy === '' ? [] : [scope.opponentBuy],
        },
      }}
    />
  );
}
