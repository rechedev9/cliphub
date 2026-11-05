import type { ReactNode } from 'react';
import Link from 'next/link';
import type { TacticalGeometry } from '@/lib/api/tactical';
import { mapSilhouette } from '@/lib/analysis/map-silhouette';
import { prettyMapName } from '@/lib/format';
import { percentLabel } from '@/lib/library/partidas';
import { cn } from '@/lib/utils';

/** Won out of played, the shape of every rate on the card. */
export type MapRate = { readonly won: number; readonly total: number };

export type MapCardProps = {
  /** Map name such as `de_mirage`. */
  map: string;
  /** Opens the matches behind the card. */
  href?: string;
  record: { readonly wins: number; readonly losses: number; readonly ties: number };
  rounds: MapRate;
  ct: MapRate;
  t: MapRate;
  /** Play-derived radar geometry for the silhouette: undefined while it loads, null when there is none. */
  geometry?: TacticalGeometry | null;
  /** Below this many rounds a rate is dimmed. */
  minRounds?: number;
  /** Extra lines under the rates, such as the last match or the vetoes. */
  footer?: ReactNode;
  /** Marks the card the scope points at. */
  selected?: boolean;
};

function Silhouette({ geometry, map }: { geometry: TacticalGeometry | null | undefined; map: string }): ReactNode {
  const shape = geometry ? mapSilhouette(geometry, 100) : null;
  // Loading keeps the box so the card does not shift; a map without geometry has no box at all.
  if (geometry !== undefined && shape === null) return null;
  return (
    <div className="flex size-16 shrink-0 items-center justify-center rounded-sm bg-surface-3">
      {shape ? (
        <svg viewBox={shape.viewBox} className="size-14 text-fg-3" role="img" aria-label={`Silueta de ${prettyMapName(map)}`}>
          <path d={shape.path} fill="currentColor" />
        </svg>
      ) : null}
    </div>
  );
}

function Rate({ label, rate, minRounds, className }: { label: string; rate: MapRate; minRounds: number; className?: string }): ReactNode {
  const low = rate.total < minRounds;
  return (
    <div className="flex flex-col gap-0.5" title={low && rate.total > 0 ? `Muestra pequeña: ${rate.total} rondas` : undefined}>
      <dt className={cn('text-meta', className ?? 'text-fg-3')}>{label}</dt>
      <dd className={cn('flex items-baseline gap-1 text-body font-medium tabular-nums', low ? 'text-fg-3' : 'text-fg-1')}>
        {percentLabel(rate.won, rate.total) || 'Sin rondas'}
        {rate.total > 0 ? (
          <>
            <span aria-hidden className="text-meta font-normal text-fg-3">
              {rate.total}
            </span>
            <span className="sr-only">de {rate.total} rondas</span>
          </>
        ) : null}
      </dd>
    </div>
  );
}

/**
 * One map of a team's record: its radar silhouette, the result record and win
 * rate, and the round rate overall and per side, each beside its round count.
 */
export function MapCard({ map, href, record, rounds, ct, t, geometry, minRounds = 20, footer, selected = false }: MapCardProps): ReactNode {
  const played = record.wins + record.losses + record.ties;
  const name = prettyMapName(map);
  return (
    <article
      aria-label={name}
      className={cn('studio-panel flex flex-col gap-3 p-4', selected && 'border-border-accent')}
    >
      <header className="flex items-center gap-3">
        <Silhouette geometry={geometry} map={map} />
        <div className="flex min-w-0 flex-col gap-0.5">
          <h3 className="truncate text-body-lg font-semibold text-fg-1">
            {href ? (
              <Link href={href} className="hover:text-primary hover:underline">
                {name}
              </Link>
            ) : (
              name
            )}
          </h3>
          <p className="text-body-sm text-fg-2 tabular-nums">
            <span className="text-outcome-win">{record.wins}</span>-<span className="text-outcome-loss">{record.losses}</span>
            {record.ties > 0 ? `-${record.ties}` : ''}
            {played > 0 ? <span className="text-fg-3">{` · ${percentLabel(record.wins, played)} victorias`}</span> : null}
          </p>
        </div>
      </header>
      <dl className="grid grid-cols-3 gap-2">
        <Rate label="Rondas" rate={rounds} minRounds={minRounds} />
        <Rate label="CT" rate={ct} minRounds={minRounds / 2} className="text-side-ct" />
        <Rate label="T" rate={t} minRounds={minRounds / 2} className="text-side-t" />
      </dl>
      {footer ? <div className="flex flex-col gap-1.5 border-t border-border-subtle pt-2 text-meta text-fg-2">{footer}</div> : null}
    </article>
  );
}
