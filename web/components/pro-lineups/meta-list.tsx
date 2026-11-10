'use client';

import type { ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import { pct } from '@/lib/api/pro-lineups';
import { grenadeIcon } from '@/lib/analysis/icons';
import { META_SMALL_SAMPLE, META_TREND_LABEL, type MetaPeriod, type MetaRow, type MetaTrend } from '@/lib/pro-lineups-meta';
import { plainTitle } from '@/lib/pro-lineups-view';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { Badge } from '@/components/ui/badge';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Segmented } from '@/components/ui/segmented';
import { cn } from '@/lib/utils';

/** How many teams a row names. */
const ROW_TEAMS = 3;

function TrendBadge({ trend }: { trend: MetaTrend | null }): ReactNode {
  if (trend === null) return null;
  const rising = trend === 'new' || trend === 'rising';
  return <Badge variant={rising ? 'success' : 'outline'}>{META_TREND_LABEL[trend]}</Badge>;
}

/** The Meta view: one side's pro grenades ranked by how often they are thrown, the teams that use them and where they are going. */
export function MetaList({
  rows,
  period,
  recentDays,
  sideRounds,
  onPeriod,
  hover,
  onHover,
  onOpen,
}: {
  rows: readonly MetaRow[];
  period: MetaPeriod;
  recentDays: number;
  /** The side's pro rounds in the period, the denominator of the rate. */
  sideRounds: number;
  onPeriod: (period: MetaPeriod) => void;
  hover: string | null;
  onHover: (id: string | null) => void;
  onOpen: (row: MetaRow) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-body font-semibold text-fg-1 tabular-nums">
          {rows.length} {rows.length === 1 ? 'granada' : 'granadas'}
        </h2>
        <Segmented
          label="Periodo"
          value={period}
          options={[
            { value: 'recent', label: `Últimos ${recentDays} días` },
            { value: 'all', label: 'Desde siempre' },
          ]}
          onChange={onPeriod}
        />
      </div>
      <p className="text-meta text-fg-3">
        Por cada 100 rondas del lado ({sideRounds} rondas pro). La tendencia compara los últimos 30 días con los 60 anteriores.
      </p>
      {rows.length === 0 ? (
        <p className="text-body-sm text-fg-2">Ninguna granada de este lado en el periodo.</p>
      ) : (
        <ol className="flex flex-col divide-y divide-border-subtle" onPointerLeave={() => onHover(null)}>
          {rows.map((row) => {
            const { lineup } = row;
            return (
              <li key={lineup.id} onPointerEnter={() => onHover(lineup.id)}>
                <button
                  type="button"
                  onClick={() => onOpen(row)}
                  className={cn(
                    'flex w-full items-start gap-2 px-2 py-1.5 text-left outline-none hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50',
                    hover === lineup.id && 'bg-surface-3',
                  )}
                >
                  <span className="w-6 shrink-0 text-right text-body-sm font-semibold text-fg-2 tabular-nums">{row.rank}</span>
                  <span aria-hidden className="mt-0.5 flex shrink-0" style={{ color: `var(${GRENADE_KIND_TOKEN[lineup.kind]})` }}>
                    <EquipmentIcon name={grenadeIcon(lineup.kind)} height={16} />
                  </span>
                  <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <span className="flex items-center gap-2">
                      <span className="truncate text-body-sm font-medium text-fg-1">{plainTitle(lineup.title)}</span>
                      <TrendBadge trend={row.trend} />
                      {row.small ? (
                        <Badge variant="outline" title={`Menos de ${META_SMALL_SAMPLE} lanzamientos en el periodo`}>
                          Poca muestra
                        </Badge>
                      ) : null}
                    </span>
                    <span className="text-meta text-fg-2 tabular-nums">
                      {row.per100.toLocaleString('es-ES')} por 100 rondas · {row.throws} lanzamientos · en el {pct(lineup.round_share)} de las rondas recientes
                    </span>
                    {lineup.teams.length === 0 ? null : (
                      <span className="truncate text-meta text-fg-3">
                        {lineup.teams
                          .slice(0, ROW_TEAMS)
                          .map((team) => (team.detail === undefined ? team.name : `${team.name} ${team.detail} %`))
                          .join(', ')}
                      </span>
                    )}
                  </span>
                  <ChevronRight aria-hidden className="mt-0.5 size-3.5 shrink-0 text-fg-3" />
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
