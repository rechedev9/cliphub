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
  type SideStatTile,
} from '@/lib/analysis/side-stats';
import { cn } from '@/lib/utils';

const ROW_TILES = SIDE_STAT_TILES.filter((tile) => tile.key !== 'maps_won');
const ROUNDS_WON = SIDE_STAT_TILES.find((tile) => tile.key === 'rounds_won');

const SIDE_NAME: Record<SideKey, string> = { t: 'T', ct: 'CT' };
const SIDE_TEXT: Record<SideKey, string> = { t: 'text-side-t', ct: 'text-side-ct' };
const SIDE_RING: Record<SideKey, string> = { t: 'border-side-t/50', ct: 'border-side-ct/50' };

const OUT_OF_FILTER = 'Fuera del filtro';

type BigSideProps = { id: SideKey; tile: SideStatTile; stat: SideStat; empty: boolean; dimmed: boolean; lit: boolean };

function BigSide({ id, tile, stat, empty, dimmed, lit }: BigSideProps): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <div
      data-side={id}
      data-dimmed={dimmed}
      className={cn(
        'flex min-w-0 flex-col items-center gap-0.5 rounded-md border bg-surface-2 px-4 py-3 text-center',
        lit ? SIDE_RING[id] : 'border-transparent',
      )}
    >
      <span className={cn('text-body font-semibold', dimmed ? 'text-fg-3' : SIDE_TEXT[id])}>{SIDE_NAME[id]}</span>
      {dimmed ? <span className="text-meta text-fg-3">{OUT_OF_FILTER}</span> : null}
      {empty ? (
        <span className="py-3 text-body font-medium text-fg-3">Sin rondas en {SIDE_NAME[id]}</span>
      ) : (
        <>
          <span className={cn('text-display font-semibold tabular-nums', dimmed ? 'text-fg-3' : 'text-fg-1')}>
            {formatSideValue(stat.value, tile.unit)}
          </span>
          <span className="text-meta text-fg-3">{tile.label}</span>
          {delta ? <span className={cn('text-body-sm font-medium tabular-nums', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
          <span className="text-meta text-fg-3 tabular-nums">{groupMeanText(stat, tile.unit)}</span>
        </>
      )}
    </div>
  );
}

type RowCellProps = { id: SideKey; tile: SideStatTile; stat: SideStat; dimmed: boolean };

function RowCell({ id, tile, stat, dimmed }: RowCellProps): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <div data-side={id} className="flex min-w-0 flex-col items-center px-2 text-center tabular-nums">
      <span className={cn('font-medium', dimmed ? 'text-fg-3' : 'text-fg-1')}>{formatSideValue(stat.value, tile.unit)}</span>
      {delta ? <span className={cn('text-meta', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
      <span className="text-meta text-fg-3">{groupMeanText(stat, tile.unit)}</span>
    </div>
  );
}

function ColumnHead({ id, dimmed, className }: { id: SideKey; dimmed: boolean; className: string }): ReactNode {
  return (
    <div className={cn('flex flex-col items-center text-center', className)}>
      <span className={cn('text-body-sm font-semibold', dimmed ? 'text-fg-3' : SIDE_TEXT[id])}>
        {SIDE_NAME[id]}
        {dimmed ? <span className="sr-only"> ({OUT_OF_FILTER.toLowerCase()})</span> : null}
      </span>
      {dimmed ? <span aria-hidden className="text-meta text-fg-3">{OUT_OF_FILTER}</span> : null}
    </div>
  );
}

function cellPlace(single: boolean, index: number): string {
  if (single) return '';
  return index === 0 ? '@3xl:col-start-1 @3xl:row-start-1' : '@3xl:col-start-3 @3xl:row-start-1';
}

export type TeamSidesCardProps = { ct: SideStats; t: SideStats; side: ReportSide };

/**
 * The Sides card of Estadísticas: T and CT round win rate large, the other figures per side between them.
 * With a side chosen, "Estadísticas generales" already shows its figures, so the table keeps only the other side.
 */
export function TeamSidesCard({ ct, t, side }: TeamSidesCardProps): ReactNode {
  const dimmed = dimmedSide(side);
  const columns = (['t', 'ct'] as const).filter((id) => dimmed === null || id === dimmed);
  const stats: Record<SideKey, SideStats> = { t, ct };
  const single = columns.length === 1;
  const rowGrid = single ? 'grid-cols-1' : 'grid-cols-2 @3xl:grid-cols-[1fr_11rem_1fr]';
  return (
    <section aria-labelledby="side-stats-sides" className="studio-panel @container flex flex-col gap-3 p-3">
      <h2 id="side-stats-sides" className="text-body font-semibold text-fg-1">
        Por lado
      </h2>
      <div className="grid items-start gap-3 @3xl:grid-cols-[11rem_minmax(0,1fr)_11rem]">
        {ROUNDS_WON ? (
          <BigSide
            id="t"
            tile={ROUNDS_WON}
            stat={t.rounds_won}
            empty={(t.rounds_played.value ?? 0) === 0}
            dimmed={dimmed === 't'}
            lit={dimmed === 'ct'}
          />
        ) : null}
        <div className="order-last flex min-w-0 flex-col @3xl:order-none">
          <div className={cn('grid gap-x-2 border-b border-border-subtle pb-1.5', rowGrid)}>
            {columns.map((id, index) => (
              <ColumnHead key={id} id={id} dimmed={dimmed === id} className={single || index === 0 ? '' : '@3xl:col-start-3'} />
            ))}
          </div>
          <ul>
            {ROW_TILES.map((tile) => (
              <li key={tile.key} className={cn('grid gap-x-2 gap-y-1 border-b border-border-subtle py-1.5 last:border-0', rowGrid)}>
                <span
                  className={cn(
                    'text-center text-body-sm text-fg-2',
                    single ? '' : 'order-first col-span-2 @3xl:order-none @3xl:col-span-1 @3xl:col-start-2 @3xl:row-start-1',
                  )}
                >
                  {tile.label}
                </span>
                {columns.map((id, index) => (
                  <div key={id} className={cellPlace(single, index)}>
                    <RowCell id={id} tile={tile} stat={stats[id][tile.key]} dimmed={dimmed === id} />
                  </div>
                ))}
              </li>
            ))}
          </ul>
        </div>
        {ROUNDS_WON ? (
          <BigSide
            id="ct"
            tile={ROUNDS_WON}
            stat={ct.rounds_won}
            empty={(ct.rounds_played.value ?? 0) === 0}
            dimmed={dimmed === 'ct'}
            lit={dimmed === 't'}
          />
        ) : null}
      </div>
    </section>
  );
}
