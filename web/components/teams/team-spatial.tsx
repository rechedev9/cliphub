'use client';

import { useState, type ReactNode } from 'react';
import type { LibraryMatch } from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import { TACTICAL_BUY_TYPES } from '@/lib/api/tactical';
import type { TacticalBuyType } from '@/lib/api/tactical';
import { DATE_RANGES, rivalOf, teamLabel } from '@/lib/library/partidas';
import { buyLabel } from '@/lib/tactical-labels';
import { ReportSideSelect } from '@/components/reports/new-report-dialog';
import { SpatialWorkspace } from '@/components/tactical/spatial/spatial-workspace';
import { ANY, OptionSelect, rangeStart } from './team-scout';

const BUYS: readonly TacticalBuyType[] = [
  TACTICAL_BUY_TYPES.full,
  TACTICAL_BUY_TYPES.force,
  TACTICAL_BUY_TYPES.semi,
  TACTICAL_BUY_TYPES.eco,
  TACTICAL_BUY_TYPES.pistol,
];

type Filters = { rival: string; range: string; side: ReportSide; buy: string };

/** A team's rounds on the 2D radar: overlaid from freeze end, or as heatmaps, one map at a time. */
export function TeamSpatial({ teamId, matches }: { teamId: string; matches: readonly LibraryMatch[] }): ReactNode {
  const [filters, setFilters] = useState<Filters>({ rival: ANY, range: ANY, side: '', buy: ANY });
  const [from] = useState(() => Date.now());
  const rivals = new Map<string, string>();
  for (const match of matches) {
    const rival = rivalOf(match, teamId);
    if (rival?.team_id) rivals.set(rival.team_id, teamLabel(rival));
  }
  const buy = BUYS.find((entry) => entry === filters.buy);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <div className="w-36">
          <ReportSideSelect value={filters.side} onChange={(side) => setFilters({ ...filters, side })} />
        </div>
        <OptionSelect
          label="Compra"
          value={filters.buy}
          options={BUYS.map((value) => ({ value, label: buyLabel(value) }))}
          onChange={(value) => setFilters({ ...filters, buy: value })}
        />
        <OptionSelect
          label="Rival"
          value={filters.rival}
          options={[...rivals].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'))}
          onChange={(rival) => setFilters({ ...filters, rival })}
        />
        <OptionSelect
          label="Fecha"
          value={filters.range}
          options={DATE_RANGES.map((range) => ({ value: range.value, label: range.label }))}
          onChange={(range) => setFilters({ ...filters, range })}
        />
      </div>
      <SpatialWorkspace
        side={filters.side}
        scope={{
          kind: 'team',
          teamId,
          query: {
            side: filters.side,
            opponent_ids: filters.rival === ANY ? [] : [filters.rival],
            from: filters.range === ANY ? undefined : rangeStart(filters.range, from),
            buys: buy === undefined ? [] : [buy],
          },
        }}
      />
    </div>
  );
}
