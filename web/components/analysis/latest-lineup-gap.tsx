'use client';

import type { ReactNode } from 'react';
import { Users } from 'lucide-react';
import { latestLineupGapText, latestLineupMissesMap } from '@/lib/analysis/lineup';
import type { MapCounts } from '@/lib/analysis/map-counts';
import type { ScopeField } from '@/lib/analysis/scope';
import type { TeamAnalysis } from '@/hooks/use-team-analysis';
import { useTeamMapCounts } from '@/hooks/use-team-map-counts';
import type { TeamScopeControl } from '@/hooks/use-team-scope-control';
import { Button } from '@/components/ui/button';

export type LatestLineupGapProps = {
  control: TeamScopeControl;
  analysis: TeamAnalysis;
  fields: readonly ScopeField[];
  mapCounts: MapCounts | null;
};

/**
 * Says why the chosen map is empty when only the default lineup hides its matches, and offers every player.
 * It counts the map again without the lineup, so it stays quiet when a date, rival or tag filter is the reason.
 */
export function LatestLineupGap({ control, analysis, fields, mapCounts }: LatestLineupGapProps): ReactNode {
  const { scope, changeScope } = control;
  const misses = fields.includes('lineup') && latestLineupMissesMap({ lineup: scope.lineup, lineups: analysis.lineups, map: scope.map, mapCounts });
  const everyone = useTeamMapCounts({ teamId: analysis.team.id, scope, fields, lineup: [], enabled: misses });
  const hidden = misses ? (everyone?.[scope.map] ?? 0) : 0;
  if (hidden === 0) return null;
  return (
    <div role="status" className="flex flex-col items-start gap-2 rounded-md border border-warning/45 bg-warning/10 px-3 py-2.5">
      <p className="text-body-sm text-fg-1">{latestLineupGapText(scope.map, hidden)}</p>
      <Button type="button" variant="outline" size="sm" onClick={() => changeScope({ lineup: { kind: 'all' } })}>
        <Users aria-hidden />
        Todos los jugadores
      </Button>
    </div>
  );
}
