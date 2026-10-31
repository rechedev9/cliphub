'use client';

import type { ReactNode } from 'react';
import { Radar } from 'lucide-react';
import type { LibraryReport } from '@/lib/api/scout';
import { EmptyState } from '@/components/records/empty-state';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';

/** The report's matches and side on the 2D radar: its rounds overlaid, or as heatmaps. */
export function ReportSpatial({ report }: { report: LibraryReport }): ReactNode {
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
      scope={{ kind: 'team', teamId: report.team_id, query: { side: report.side, match_ids: report.match_ids } }}
    />
  );
}
