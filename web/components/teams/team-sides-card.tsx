'use client';

import type { ReactNode } from 'react';
import type { ReportSide } from '@/lib/api/scout';
import type { SideStat, SideStatKey, SideStats } from '@/lib/api/side-stats';
import { DELTA_TONE_CLASS } from '@/components/ui/delta-value';
import { SIDE_STAT_TILES, dimmedSide, formatSideValue, sideDelta, type SideKey, type SideStatTile } from '@/lib/analysis/side-stats';
import { prettyMapName } from '@/lib/format';
import { MapThumb } from '@/components/analysis/map-thumb';
import { cn } from '@/lib/utils';

/** EDGE's row order: the pistol first, the utility last. */
const ROW_KEYS: readonly SideStatKey[] = ['pistols_won', 'opening_kills', 'trades', 'won_4v5', 'won_5v4', 'flash_assists_per_round', 'utility_damage_per_round'];
const ROWS = ROW_KEYS.flatMap((key) => SIDE_STAT_TILES.filter((tile) => tile.key === key));
const ROUNDS_WON = SIDE_STAT_TILES.find((tile) => tile.key === 'rounds_won');

const SIDE_NAME: Record<SideKey, string> = { t: 'T', ct: 'CT' };
const SIDE_TEXT: Record<SideKey, string> = { t: 'text-side-t', ct: 'text-side-ct' };
const SIDE_LINE: Record<SideKey, string> = { t: 'border-side-t', ct: 'border-side-ct' };
const SIDE_TINT: Record<SideKey, string> = {
  t: 'bg-linear-to-r from-side-t/25 via-surface-2/70 to-surface-2',
  ct: 'bg-linear-to-l from-side-ct/20 via-surface-2/70 to-surface-2',
};
const CAPS = 'text-meta font-bold tracking-wider uppercase';

const VERSUS: Record<'better' | 'worse' | 'even', string> = {
  better: 'por encima de la media',
  worse: 'por debajo de la media',
  even: 'en la media',
};

function deltaTitle(stat: SideStat, tile: SideStatTile): string {
  if (stat.group_avg === null) return 'Sin media del grupo';
  return `Media del grupo ${formatSideValue(stat.group_avg, tile.unit)}`;
}

/** One side's half of the banner: the map picture tinted in the side's colour, its round win rate and the rounds behind it. */
function SidePanel({ id, stats, map, dimmed }: { id: SideKey; stats: SideStats; map: string; dimmed: boolean }): ReactNode {
  const rate = stats.rounds_won;
  const delta = ROUNDS_WON === undefined ? null : sideDelta(rate, ROUNDS_WON.unit);
  const played = stats.rounds_played.value ?? 0;
  const right = id === 'ct';
  return (
    <div className={cn('relative isolate flex min-h-56 flex-col justify-between gap-6 overflow-hidden p-4', right && 'items-end text-right', dimmed && 'opacity-35')}>
      {map === '' ? null : <MapThumb map={map} photo className="absolute inset-0 -z-20 size-full rounded-none opacity-50" />}
      <div aria-hidden className={cn('absolute inset-0 -z-10', SIDE_TINT[id])} />
      <div className="flex flex-col gap-0.5">
        <span className={cn(CAPS, 'text-fg-3')}>{map === '' ? 'Todos los mapas' : prettyMapName(map)}</span>
        <span className={cn(CAPS, 'text-fg-2')}>Rondas ganadas</span>
        <span className="font-num text-display leading-none font-bold text-fg-1 tabular-nums">
          {ROUNDS_WON === undefined ? '' : formatSideValue(rate.value, ROUNDS_WON.unit)}
        </span>
        {delta ? (
          <span className={cn('text-body-sm font-semibold tabular-nums', DELTA_TONE_CLASS[delta.tone])}>
            {delta.text} {VERSUS[delta.tone]}
          </span>
        ) : null}
      </div>
      <div className="flex flex-col">
        <span className="font-num text-body font-bold text-fg-1 tabular-nums">{played}</span>
        <span className={cn(CAPS, 'text-fg-2')}>Rondas jugadas</span>
      </div>
      {dimmed ? <span className="sr-only">Fuera del filtro</span> : null}
    </div>
  );
}

function Value({ stat, tile, dimmed, align }: { stat: SideStat; tile: SideStatTile; dimmed: boolean; align: 'start' | 'end' }): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  const arrow = delta === null || delta.tone === 'even' ? '' : delta.text.slice(0, 1);
  return (
    <span
      title={deltaTitle(stat, tile)}
      className={cn('flex items-center gap-1.5 font-num text-body font-bold tabular-nums', align === 'end' ? 'justify-end' : 'justify-start', dimmed ? 'text-fg-4' : 'text-fg-1')}
    >
      {align === 'end' ? <span className={cn('text-[0.5rem]', delta ? DELTA_TONE_CLASS[delta.tone] : '')}>{arrow}</span> : null}
      {formatSideValue(stat.value, tile.unit)}
      {align === 'start' ? <span className={cn('text-[0.5rem]', delta ? DELTA_TONE_CLASS[delta.tone] : '')}>{arrow}</span> : null}
    </span>
  );
}

export type TeamSidesCardProps = {
  ct: SideStats;
  t: SideStats;
  side: ReportSide;
  /** The scope's map; its picture fills both halves. */
  map: string;
};

/**
 * EDGE's Sides banner: T at the left and CT at the right, each with its round win rate over the map
 * picture, and between them the two sides' figures row by row around the label. The side the filter
 * leaves out is dimmed.
 */
export function TeamSidesCard({ ct, t, side, map }: TeamSidesCardProps): ReactNode {
  const dimmed = dimmedSide(side);
  return (
    <section aria-label="Lados" className="@container overflow-hidden border border-border bg-surface-2">
      <div className="grid @3xl:grid-cols-[minmax(0,1fr)_minmax(0,1.6fr)_minmax(0,1fr)]">
        <SidePanel id="t" stats={t} map={map} dimmed={dimmed === 't'} />
        <div className="flex min-w-0 flex-col justify-center px-3 py-3">
          <div className="grid grid-cols-[4.5rem_minmax(0,1fr)_4.5rem] items-end gap-x-2 pb-1">
            {(['t', 'ct'] as const).map((id) => (
              <span
                key={id}
                className={cn(
                  CAPS,
                  'w-fit border-b-2 pb-0.5',
                  id === 'ct' ? 'col-start-3 justify-self-end' : 'col-start-1',
                  dimmed === id ? 'border-transparent text-fg-4' : cn(SIDE_TEXT[id], SIDE_LINE[id]),
                )}
              >
                Lado {SIDE_NAME[id]}
              </span>
            ))}
          </div>
          <ul>
            {ROWS.map((tile) => (
              <li key={tile.key} className="grid grid-cols-[4.5rem_minmax(0,1fr)_4.5rem] items-center gap-x-2 border-b border-border-subtle py-2 last:border-0">
                <Value stat={t[tile.key]} tile={tile} dimmed={dimmed === 't'} align="end" />
                <span className="text-center text-[0.625rem] leading-tight font-bold tracking-wider text-fg-2 uppercase">{tile.label}</span>
                <Value stat={ct[tile.key]} tile={tile} dimmed={dimmed === 'ct'} align="start" />
              </li>
            ))}
          </ul>
        </div>
        <SidePanel id="ct" stats={ct} map={map} dimmed={dimmed === 'ct'} />
      </div>
    </section>
  );
}
