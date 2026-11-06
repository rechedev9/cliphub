import type { ReactNode } from 'react';
import { compareDelta, deltaArrow } from '@/lib/analysis/compare';
import type { Reliability } from '@/lib/analysis/reference';
import { cn } from '@/lib/utils';
import { DELTA_TONE_CLASS } from './delta-value';
import { Sparkline } from './sparkline';

export type StatComparison = {
  /** The reference value; null when it cannot be computed for this scope. */
  readonly value: number | null;
  /** What the reference is, such as "Media del resto de jugadores". */
  readonly label: string;
};

export type StatCardProps = {
  label: string;
  /** null when the scope has no data for it. */
  value: number | null;
  format: (value: number) => string;
  comparison?: StatComparison;
  /** Which direction is good news; sets the colour of the delta. Default true. */
  higherIsBetter?: boolean;
  /** Formats the signed difference; default is `format` with a sign. */
  formatDelta?: (delta: number) => string;
  /** Differences smaller than this read as even. */
  epsilon?: number;
  /** One value per match, oldest first, drawn as a sparkline. */
  trend?: readonly number[];
  /** Sample size under the value, such as "238 rondas". */
  sample?: string;
  /** The shrunk estimate: arrow and colour only when its interval leaves the reference out. */
  reliability?: Reliability | null;
  className?: string;
};

function signed(format: (value: number) => string): (delta: number) => string {
  return (delta) => {
    let sign = '';
    if (delta > 0) sign = '+';
    else if (delta < 0) sign = '-';
    return `${sign}${format(Math.abs(delta))}`;
  };
}

/**
 * One headline number with what it is compared against, dense: the value, then
 * "▲ +0,05 vs 0,71". The arrow gives the direction and its colour whether that
 * is good news, so `higherIsBetter` decides the colour, not the sign. Group
 * cards in a `StatCardGroup`.
 */
export function StatCard({
  label,
  value,
  format,
  comparison,
  higherIsBetter = true,
  formatDelta,
  epsilon,
  trend,
  sample,
  reliability,
  className,
}: StatCardProps): ReactNode {
  const result =
    value !== null && comparison !== undefined && comparison.value !== null
      ? compareDelta(value, comparison.value, { higherIsBetter, epsilon })
      : null;
  // A difference the interval cannot tell from noise keeps its number but loses the arrow and the colour.
  const called = result !== null && result.tone !== 'even' && (reliability ? reliability.differs : true);
  const deltaText = formatDelta ?? signed(format);
  return (
    <div className={cn('row-span-2 grid min-w-0 grid-rows-subgrid gap-y-0.5 bg-surface-2 px-3 py-2', className)}>
      <dt className="self-end text-meta leading-tight text-balance text-fg-3">{label}</dt>
      <dd className="flex flex-col justify-start gap-0.5">
        <span className="flex flex-wrap items-center justify-between gap-x-2 gap-y-0.5">
          <span className={cn('text-title font-semibold whitespace-nowrap text-fg-1 tabular-nums', value === null && 'text-body-sm font-normal text-fg-3')}>
            {value === null ? 'Sin dato' : format(value)}
          </span>
          {trend ? <Sparkline values={trend} label={`${label} por partida`} format={format} width={64} height={20} /> : null}
        </span>
        {result && comparison && comparison.value !== null ? (
          <span
            className="text-meta text-fg-3 tabular-nums"
            title={`${comparison.label}: ${format(comparison.value)}.${reliability ? ` ${reliability.detail}` : ''}`}
          >
            <span className={cn('font-medium whitespace-nowrap', called ? DELTA_TONE_CLASS[result.tone] : 'text-fg-2')}>
              {result.tone === 'even' ? '= ' : `${called ? `${deltaArrow(result.delta, epsilon)} ` : ''}${deltaText(result.delta)}`}
            </span>{' '}
            <span className="whitespace-nowrap">vs {format(comparison.value)}</span>
            <span className="sr-only">
              , {comparison.label.toLowerCase()}
              {reliability ? `. ${reliability.detail}` : ''}
            </span>
          </span>
        ) : null}
        {reliability?.weak ? <span className="text-meta text-fg-3">Poca muestra</span> : null}
        {sample ? <span className="text-meta text-fg-3">{sample}</span> : null}
      </dd>
    </div>
  );
}

/** Stat cards on one surface, as many per row as fit; labels share a row so the values line up. */
export function StatCardGroup({ label, children, className }: { label: string; children: ReactNode; className?: string }): ReactNode {
  return (
    <dl
      aria-label={label}
      className={cn('studio-panel grid grid-cols-[repeat(auto-fit,minmax(7rem,1fr))] gap-px overflow-hidden bg-border-subtle', className)}
    >
      {children}
    </dl>
  );
}
