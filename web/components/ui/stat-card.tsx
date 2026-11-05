import type { ReactNode } from 'react';
import { compareDelta, type DeltaTone } from '@/lib/analysis/compare';
import { cn } from '@/lib/utils';
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

const TONE_CLASS: Record<DeltaTone, string> = {
  better: 'text-outcome-win',
  worse: 'text-outcome-loss',
  even: 'text-fg-2',
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
 * One headline number with what it is compared against. The delta is green
 * when the value is better than the reference and red when worse, so
 * `higherIsBetter` decides the colour, not the sign. Group cards in a
 * `StatCardGroup`.
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
    <div className={cn('flex min-w-0 flex-col gap-1.5 bg-surface-2 px-4 py-3', className)}>
      <dt className="text-meta text-fg-3">{label}</dt>
      <dd className="flex flex-col gap-1">
        <span className="flex items-end justify-between gap-3">
          <span className={cn('text-stat font-semibold whitespace-nowrap text-fg-1 tabular-nums', value === null && 'text-fg-3')}>
            {value === null ? 'Sin dato' : format(value)}
          </span>
          {trend ? <Sparkline values={trend} label={`${label} por partida`} format={format} /> : null}
        </span>
        {result && comparison && comparison.value !== null ? (
          <span className="text-meta text-fg-2">
            <span className={cn('font-medium tabular-nums', TONE_CLASS[result.tone])}>
              {result.tone === 'even' ? 'Igual' : deltaText(result.delta)}
            </span>{' '}
            frente a {format(comparison.value)}, {comparison.label.toLowerCase()}
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
      className={cn('studio-panel grid grid-cols-[repeat(auto-fit,minmax(13rem,1fr))] gap-px overflow-hidden bg-border-subtle', className)}
    >
      {children}
    </dl>
  );
}
