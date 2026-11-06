import type { ReactNode } from 'react';
import { compareDelta, deltaArrow } from '@/lib/analysis/compare';
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
  className,
}: StatCardProps): ReactNode {
  const result =
    value !== null && comparison !== undefined && comparison.value !== null
      ? compareDelta(value, comparison.value, { higherIsBetter, epsilon })
      : null;
  const deltaText = formatDelta ?? signed(format);
  return (
    <div className={cn('flex min-w-0 flex-col gap-0.5 bg-surface-2 px-3 py-2', className)}>
      <dt className="truncate text-meta text-fg-3" title={label}>
        {label}
      </dt>
      <dd className="flex flex-col gap-0.5">
        <span className="flex items-center justify-between gap-2">
          <span className={cn('text-title font-semibold whitespace-nowrap text-fg-1 tabular-nums', value === null && 'text-body-sm font-normal text-fg-3')}>
            {value === null ? 'Sin dato' : format(value)}
          </span>
          {trend ? <Sparkline values={trend} label={`${label} por partida`} format={format} width={64} height={20} /> : null}
        </span>
        {result && comparison && comparison.value !== null ? (
          <span className="truncate text-meta text-fg-3 tabular-nums" title={`${comparison.label}: ${format(comparison.value)}`}>
            <span className={cn('font-medium', DELTA_TONE_CLASS[result.tone])}>
              {result.tone === 'even' ? '= ' : `${deltaArrow(result.delta, epsilon)} ${deltaText(result.delta)}`}
            </span>{' '}
            vs {format(comparison.value)}
            <span className="sr-only">, {comparison.label.toLowerCase()}</span>
          </span>
        ) : null}
        {sample ? <span className="text-meta text-fg-3">{sample}</span> : null}
      </dd>
    </div>
  );
}

/** Stat cards on one surface, as many per row as fit. */
export function StatCardGroup({ label, children, className }: { label: string; children: ReactNode; className?: string }): ReactNode {
  return (
    <dl
      aria-label={label}
      className={cn('studio-panel grid grid-cols-[repeat(auto-fit,minmax(7.5rem,1fr))] gap-px overflow-hidden bg-border-subtle', className)}
    >
      {children}
    </dl>
  );
}
