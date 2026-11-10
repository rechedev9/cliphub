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

function dimClass(dimmed: boolean): string {
  return dimmed ? 'opacity-45 grayscale' : '';
}

type BigSideProps = { id: SideKey; tile: SideStatTile; stat: SideStat; dimmed: boolean; lit: boolean };

function BigSide({ id, tile, stat, dimmed, lit }: BigSideProps): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <div
      data-side={id}
      data-dimmed={dimmed}
      className={cn(
        'flex min-w-0 flex-col items-center gap-0.5 rounded-md border bg-surface-2 px-4 py-3 text-center',
        lit ? SIDE_RING[id] : 'border-transparent',
        dimClass(dimmed),
      )}
    >
      <span className={cn('text-body font-semibold', SIDE_TEXT[id])}>{SIDE_NAME[id]}</span>
      <span className="text-display font-semibold text-fg-1 tabular-nums">{formatSideValue(stat.value, tile.unit)}</span>
      <span className="text-meta text-fg-3">{tile.label}</span>
      {delta ? <span className={cn('text-body-sm font-medium tabular-nums', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
      <span className="text-meta text-fg-3 tabular-nums">{groupMeanText(stat, tile.unit)}</span>
    </div>
  );
}

type RowCellProps = { id: SideKey; tile: SideStatTile; stat: SideStat; dimmed: boolean };

function RowCell({ id, tile, stat, dimmed }: RowCellProps): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <td data-side={id} className={cn('px-2 py-1.5 text-center tabular-nums', dimClass(dimmed))}>
      <span className="font-medium text-fg-1">{formatSideValue(stat.value, tile.unit)}</span>
      {delta ? <span className={cn('ml-2 text-meta', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
      <span className="block text-meta text-fg-3">{groupMeanText(stat, tile.unit)}</span>
    </td>
  );
}

export type TeamSidesCardProps = { ct: SideStats; t: SideStats; side: ReportSide };

/** The Sides card of Estadísticas: T and CT round win rate large, the other figures per side between them. */
export function TeamSidesCard({ ct, t, side }: TeamSidesCardProps): ReactNode {
  const dimmed = dimmedSide(side);
  return (
    <section aria-labelledby="side-stats-sides" className="studio-panel flex flex-col gap-3 p-3">
      <h2 id="side-stats-sides" className="text-body font-semibold text-fg-1">
        Por lado
      </h2>
      <div className="grid items-center gap-3 md:grid-cols-[11rem_minmax(0,1fr)_11rem]">
        {ROUNDS_WON ? <BigSide id="t" tile={ROUNDS_WON} stat={t.rounds_won} dimmed={dimmed === 't'} lit={dimmed === 'ct'} /> : null}
        <table className="order-last w-full text-body-sm md:order-none">
          <thead>
            <tr className="border-b border-border-subtle text-meta">
              <th scope="col" className={cn('w-1/3 px-2 py-1.5 text-center font-semibold', SIDE_TEXT.t, dimClass(dimmed === 't'))}>
                T
              </th>
              <th scope="col" className="px-2 py-1.5 text-center font-normal text-fg-3">
                Cifra
              </th>
              <th scope="col" className={cn('w-1/3 px-2 py-1.5 text-center font-semibold', SIDE_TEXT.ct, dimClass(dimmed === 'ct'))}>
                CT
              </th>
            </tr>
          </thead>
          <tbody>
            {ROW_TILES.map((tile) => (
              <tr key={tile.key} className="border-b border-border-subtle last:border-0">
                <RowCell id="t" tile={tile} stat={t[tile.key]} dimmed={dimmed === 't'} />
                <th scope="row" className="px-2 py-1.5 text-center font-normal text-fg-2">
                  {tile.label}
                </th>
                <RowCell id="ct" tile={tile} stat={ct[tile.key]} dimmed={dimmed === 'ct'} />
              </tr>
            ))}
          </tbody>
        </table>
        {ROUNDS_WON ? <BigSide id="ct" tile={ROUNDS_WON} stat={ct.rounds_won} dimmed={dimmed === 'ct'} lit={dimmed === 't'} /> : null}
      </div>
    </section>
  );
}
