'use client';

import type { ReactNode } from 'react';
import type { ReportSide } from '@/lib/api/scout';
import type { SideStat, SideStats } from '@/lib/api/side-stats';
import { DELTA_TONE_CLASS } from '@/components/ui/delta-value';
import {
  SIDE_STAT_TILES,
  dimmedSide,
  formatSideValue,
  groupMeanText,
  sideDelta,
  type SideKey,
} from '@/lib/analysis/side-stats';
import { prettyMapName } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import { cn } from '@/lib/utils';

const ROUNDS_WON = SIDE_STAT_TILES.find((tile) => tile.key === 'rounds_won');
const ROW_TILES = SIDE_STAT_TILES.filter((tile) => tile.key !== 'maps_won' && tile.key !== 'rounds_won');

const SIDE_NAME: Record<SideKey, string> = { t: 'T', ct: 'CT' };
const SIDE_TEXT: Record<SideKey, string> = { t: 'text-side-t', ct: 'text-side-ct' };
const CAPS = 'text-meta font-medium tracking-wider text-fg-3 uppercase';
const OUT_OF_FILTER = 'Fuera del filtro';

const VERSUS: Record<'better' | 'worse' | 'even', string> = {
  better: 'por encima de la media',
  worse: 'por debajo de la media',
  even: 'en la media',
};

function SideColumnHead({ id, dimmed }: { id: SideKey; dimmed: boolean }): ReactNode {
  return (
    <span className={cn('text-center text-meta font-bold tracking-wider uppercase', dimmed ? 'text-fg-4' : SIDE_TEXT[id])}>
      Lado {SIDE_NAME[id]}
      {dimmed ? <span className="sr-only"> ({OUT_OF_FILTER.toLowerCase()})</span> : null}
    </span>
  );
}

function RowCell({ stat, unit, dimmed }: { stat: SideStat; unit: (typeof SIDE_STAT_TILES)[number]['unit']; dimmed: boolean }): ReactNode {
  const delta = sideDelta(stat, unit);
  return (
    <span title={groupMeanText(stat, unit)} className={cn('flex min-w-0 flex-col items-center tabular-nums', dimmed && 'opacity-45')}>
      <span className="font-bold text-fg-1">{formatSideValue(stat.value, unit)}</span>
      {delta ? <span className={cn('text-meta', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
    </span>
  );
}

export type TeamSidesCardProps = {
  ct: SideStats;
  t: SideStats;
  /** The scope's own figures, the ones the big rate and its delta read. */
  overall: SideStats;
  side: ReportSide;
  /** The scope's map; its photo fades into the banner. */
  map: string;
};

/**
 * The "Lados" banner: the round win rate large with its delta to the group, the group's own mean mirrored
 * at the right, and between them the T and CT figures row by row. A side left out of the filter is dimmed.
 */
export function TeamSidesCard({ ct, t, overall, side, map }: TeamSidesCardProps): ReactNode {
  const dimmed = dimmedSide(side);
  const stats: Record<SideKey, SideStats> = { t, ct };
  const rate = overall.rounds_won;
  const delta = ROUNDS_WON === undefined ? null : sideDelta(rate, ROUNDS_WON.unit);
  const played = overall.rounds_played.value ?? 0;
  return (
    <section aria-label="Lados" className="relative @container isolate overflow-hidden border border-border bg-surface-2">
      {map === '' ? null : <MapThumb map={map} className="absolute inset-0 -z-20 size-full rounded-none opacity-35" crop={1.1} />}
      <div aria-hidden className="absolute inset-0 -z-10 bg-linear-to-r from-surface-2 via-surface-2/70 to-surface-2" />
      <div className="grid items-center gap-4 p-4 @3xl:grid-cols-[11rem_minmax(0,1fr)_11rem]">
        <div className="flex flex-col gap-0.5">
          <span className="text-body font-bold tracking-wider text-fg-1 uppercase">{map === '' ? 'Todos los mapas' : prettyMapName(map)}</span>
          <span className={CAPS}>Rondas ganadas</span>
          <span className="text-display leading-none font-bold text-fg-1 tabular-nums">
            {ROUNDS_WON === undefined ? '' : formatSideValue(rate.value, ROUNDS_WON.unit)}
          </span>
          {delta ? (
            <span className={cn('text-body-sm font-medium tabular-nums', DELTA_TONE_CLASS[delta.tone])}>
              {delta.text} {VERSUS[delta.tone]}
            </span>
          ) : null}
          <span className={cn(CAPS, 'mt-1')}>{played === 1 ? '1 ronda jugada' : `${played} rondas jugadas`}</span>
        </div>
        <div className="flex min-w-0 flex-col gap-1">
          <div className="grid grid-cols-[minmax(0,1fr)_5rem_5rem] items-end gap-x-2 border-b border-border-subtle pb-1.5">
            <span />
            <SideColumnHead id="t" dimmed={dimmed === 't'} />
            <SideColumnHead id="ct" dimmed={dimmed === 'ct'} />
          </div>
          <ul>
            {ROW_TILES.map((tile) => (
              <li key={tile.key} className="grid grid-cols-[minmax(0,1fr)_5rem_5rem] items-center gap-x-2 border-b border-border-subtle py-1.5 text-body-sm last:border-0">
                <span className={cn(CAPS, 'truncate')}>{tile.label}</span>
                <RowCell stat={stats.t[tile.key]} unit={tile.unit} dimmed={dimmed === 't'} />
                <RowCell stat={stats.ct[tile.key]} unit={tile.unit} dimmed={dimmed === 'ct'} />
              </li>
            ))}
          </ul>
        </div>
        <div className="hidden flex-col items-end gap-0.5 text-right opacity-60 @3xl:flex">
          <span className={CAPS}>Media del grupo</span>
          <span className="text-display leading-none font-bold text-fg-1 tabular-nums">
            {ROUNDS_WON === undefined ? '' : formatSideValue(rate.group_avg, ROUNDS_WON.unit)}
          </span>
          <span className="text-meta text-fg-3 tabular-nums">{ROUNDS_WON === undefined ? '' : groupMeanText(rate, ROUNDS_WON.unit)}</span>
        </div>
      </div>
    </section>
  );
}
