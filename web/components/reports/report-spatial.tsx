'use client';

import { useCallback, type ReactNode } from 'react';
import { Radar } from 'lucide-react';
import type { LibraryReport } from '@/lib/api/scout';
import type { SpatialResponse } from '@/lib/api/spatial';
import type { AnalysisScope } from '@/lib/analysis/scope';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { EmptyState } from '@/components/records/empty-state';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

/**
 * The report's matches and side on the 2D radar: its rounds overlaid, or as
 * heatmaps. The page's scope picks the map and the buys; the side is the report's.
 */
export function ReportSpatial({
  report,
  scope,
  onMapChange,
  onCounts,
}: {
  report: LibraryReport;
  scope: AnalysisScope;
  onMapChange: (map: string) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const onLoaded = useCallback(
    (response: SpatialResponse) => {
      onCounts({ maps: response.matches.length, rounds: response.heatmap?.rounds ?? response.overlay?.total ?? 0 });
    },
    [onCounts],
  );
  if (report.match_ids.length === 0) {
    return (
      <EmptyState
        icon={Radar}
        title="El informe no tiene partidas"
        description="Edita el informe y elige las partidas del rival para verlas en el radar."
      />
    );
  }
  return (
    <SpatialWorkspace
      side={report.side}
      onMapChange={onMapChange}
      onLoaded={onLoaded}
      scope={{
        kind: 'team',
        teamId: report.team_id,
        query: {
          map: scope.map === '' ? undefined : scope.map,
          side: report.side,
          match_ids: report.match_ids,
          buys: scope.buy === '' ? [] : [scope.buy],
          opponent_buys: scope.opponentBuy === '' ? [] : [scope.opponentBuy],
        },
      }}
    />
  );
}
