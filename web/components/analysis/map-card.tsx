import type { ReactNode } from 'react';
import Link from 'next/link';
import type { TacticalGeometry } from '@/lib/api/tactical';
import { prettyMapName } from '@/lib/format';
import { percentLabel } from '@/lib/library/partidas';
import { MapThumb } from '@/components/analysis/map-thumb';
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
  /** Play-derived radar geometry for the fallback silhouette when CS2 is missing. */
  geometry?: TacticalGeometry | null;
  /** Below this many rounds a rate is dimmed. */
  minRounds?: number;
  /** Extra lines under the rates, such as the last match or the vetoes. */
  footer?: ReactNode;
  /** Marks the card the scope points at. */
  selected?: boolean;
};

function Rate({ label, rate, minRounds, className }: { label: string; rate: MapRate; minRounds: number; className?: string }): ReactNode {
  const low = rate.total < minRounds;
  return (
    <div className="flex flex-col" title={low && rate.total > 0 ? `Muestra pequeña: ${rate.total} rondas` : undefined}>
      <dt className={cn('text-meta', className ?? 'text-fg-3')}>{label}</dt>
      <dd className={cn('flex items-baseline gap-1 text-body-sm font-semibold tabular-nums', low ? 'text-fg-3' : 'text-fg-1')}>
        {percentLabel(rate.won, rate.total) || 'Sin rondas'}
        {rate.total > 0 ? (
          <>
            <span aria-hidden className="text-meta font-normal text-fg-3">
              ({rate.total})
            </span>
            <span className="sr-only">de {rate.total} rondas</span>
          </>
        ) : null}
      </dd>
    </div>
  );
}

/**
 * One map of a team's record: a strip of CS2's radar with the name and result
 * over it, then the round rate overall and per side, each with its rounds.
 */
export function MapCard({ map, href, record, rounds, ct, t, geometry, minRounds = 20, footer, selected = false }: MapCardProps): ReactNode {
  const played = record.wins + record.losses + record.ties;
  const name = prettyMapName(map);
  return (
    <article aria-label={name} className={cn('studio-panel flex flex-col overflow-hidden', selected && 'border-border-accent')}>
      <header className="relative flex h-20 items-end">
        <MapThumb map={map} geometry={geometry} className="absolute inset-0 size-full rounded-none" crop={1.1} />
        <span aria-hidden className="absolute inset-x-0 bottom-0 h-9 bg-surface-0/70" />
        <div className="relative flex w-full items-end justify-between gap-2 px-3 pb-2">
          <h3 className="truncate text-body font-semibold text-fg-1">
            {href ? (
              <Link href={href} className="hover:underline">
                {name}
              </Link>
            ) : (
              name
            )}
          </h3>
          <p className="text-body-sm font-medium whitespace-nowrap text-fg-1 tabular-nums">
            <span className="text-outcome-win">{record.wins}</span>-<span className="text-outcome-loss">{record.losses}</span>
            {record.ties > 0 ? `-${record.ties}` : ''}
            {played > 0 ? <span className="font-normal text-fg-2">{` · ${percentLabel(record.wins, played)}`}</span> : null}
          </p>
        </div>
      </header>
      <dl className="grid grid-cols-3 gap-2 px-3 py-2">
        <Rate label="Rondas" rate={rounds} minRounds={minRounds} />
        <Rate label="CT" rate={ct} minRounds={minRounds / 2} className="text-side-ct" />
        <Rate label="T" rate={t} minRounds={minRounds / 2} className="text-side-t" />
      </dl>
      {footer ? <div className="flex flex-col gap-1 border-t border-border-subtle px-3 py-2 text-meta text-fg-2">{footer}</div> : null}
    </article>
  );
}
