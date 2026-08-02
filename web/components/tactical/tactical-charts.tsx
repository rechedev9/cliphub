'use client';

import { useId, type ReactNode } from 'react';
import { MIN_RELIABLE_SAMPLE } from '@/lib/api/tactical';
import type { TacticalHistogram, TacticalRate } from '@/lib/api/tactical';
import { rateLabel } from '@/lib/tactical-labels';
import { cn } from '@/lib/utils';

/**
 * The small charts of the tendencies panel, in SVG.
 *
 * One rule runs through all of them: a percentage never appears without the
 * count it came from, and a rate the aggregate flagged as unreliable is drawn
 * hatched and labelled as a small sample instead of as a clean number. A
 * scouting report that hides its denominator is how analysts get misled.
 */

/** Marks a rate the aggregate computed from fewer than MIN_RELIABLE_SAMPLE rounds. */
export function SmallSampleChip({ className }: { className?: string }): ReactNode {
  return (
    <span
      title={`Menos de ${MIN_RELIABLE_SAMPLE} rondas: no es una tendencia`}
      className={cn(
        // The house pair for a tinted chip: edge 45%, fill 10%, square.
        'shrink-0 border border-warning/45 bg-warning/10 px-2 py-0.5 font-mono text-meta uppercase tracking-wider text-warning',
        className,
      )}
    >
      muestra baja
    </span>
  );
}

/** A rate as a bar; an unreliable one is hatched so it can never read as solid. */
export function RateBar({ rate, className }: { rate: TacticalRate; className?: string }): ReactNode {
  const patternId = useId();
  const pct = Math.max(0, Math.min(100, rate.pct));
  return (
    <svg
      width="100%"
      height="8"
      role="presentation"
      className={cn('block overflow-hidden rounded-sm', className)}
    >
      <defs>
        <pattern
          id={patternId}
          width="5"
          height="5"
          patternUnits="userSpaceOnUse"
          patternTransform="rotate(45)"
        >
          <rect width="5" height="5" fill="currentColor" opacity="0.18" />
          <line x1="0" y1="0" x2="0" y2="5" stroke="currentColor" strokeWidth="2.5" />
        </pattern>
      </defs>
      <rect width="100%" height="8" rx="1.5" className="fill-surface-3" />
      <rect
        width={`${pct}%`}
        height="8"
        rx="1.5"
        fill={rate.reliable ? 'currentColor' : `url(#${patternId})`}
      />
    </svg>
  );
}

/** The number, always with its denominator. */
export function RateValue({ rate, className }: { rate: TacticalRate; className?: string }): ReactNode {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <span
        className={cn(
          'font-mono text-meta tracking-normal tabular-nums',
          rate.reliable ? 'text-foreground' : 'text-muted-foreground',
        )}
      >
        {rateLabel(rate.pct, rate.total)}
      </span>
      {rate.total > 0 && !rate.reliable ? <SmallSampleChip /> : null}
    </span>
  );
}

/** Bar colour by meaning: share is neutral cyan, a win rate is a result. */
const RATE_TONE = {
  primary: 'text-primary',
  success: 'text-success',
  muted: 'text-muted-foreground',
} as const;

/** One labelled rate: bar plus number, the row every distribution is built from. */
export function RateRow({
  label,
  rate,
  tone = 'primary',
  meta,
}: {
  label: ReactNode;
  rate: TacticalRate;
  tone?: keyof typeof RATE_TONE;
  meta?: ReactNode;
}): ReactNode {
  const toneClass = RATE_TONE[tone];
  return (
    <div className="grid grid-cols-[minmax(96px,1fr)_minmax(0,2fr)_auto] items-center gap-x-3 gap-y-1">
      <span className="min-w-0 truncate font-mono text-meta uppercase tracking-wider text-fg-3">
        {label}
      </span>
      <RateBar rate={rate} className={toneClass} />
      <span className="flex items-center gap-2 justify-self-end">
        {meta}
        <RateValue rate={rate} />
      </span>
    </div>
  );
}

const HISTOGRAM_HEIGHT = 88;
const HISTOGRAM_BASE = 72;

/**
 * A timing distribution in seconds from freeze-time end, with the median kept
 * alongside because a median survives the long tail that a mean does not.
 */
export function HistogramChart({
  histogram,
  title,
}: {
  histogram: TacticalHistogram;
  title: string;
}): ReactNode {
  const buckets = histogram.buckets;
  const peak = buckets.reduce((max, bucket) => Math.max(max, bucket.count), 0);
  const span = buckets.length;
  // The axis runs from zero to the end of the last bucket, so a bar's x is its
  // share of that span and the chart needs no viewBox scaling to fit its box.
  const axisSeconds = span > 0 ? buckets[span - 1].from_seconds + histogram.bucket_seconds : 0;
  const bucketPct = axisSeconds > 0 ? (histogram.bucket_seconds / axisSeconds) * 100 : 0;
  const medianPct = axisSeconds > 0 ? (histogram.median / axisSeconds) * 100 : 0;

  return (
    <figure className="flex min-w-0 flex-col gap-2">
      <figcaption className="flex items-baseline justify-between gap-3">
        <span className="font-mono text-meta uppercase tracking-wider text-fg-3">{title}</span>
        <span className="flex items-center gap-2">
          <span className="font-mono text-meta tracking-normal tabular-nums text-foreground">
            mediana {histogram.median.toFixed(1)} s (n={histogram.samples})
          </span>
          {histogram.samples > 0 && histogram.samples < MIN_RELIABLE_SAMPLE ? <SmallSampleChip /> : null}
        </span>
      </figcaption>

      {histogram.samples === 0 || peak === 0 ? (
        <p className="rounded-md border border-border bg-surface-1 px-3 py-4 text-center font-mono text-meta uppercase tracking-wider text-fg-3">
          sin muestras
        </p>
      ) : (
        <svg
          width="100%"
          height={HISTOGRAM_HEIGHT}
          role="img"
          aria-label={`${title}: mediana ${histogram.median.toFixed(1)} segundos sobre ${histogram.samples} muestras`}
          className="block"
        >
          <line
            x1="0"
            y1={HISTOGRAM_BASE}
            x2="100%"
            y2={HISTOGRAM_BASE}
            className="stroke-border"
            strokeWidth="1"
          />
          {buckets.map((bucket) => {
            const height = (bucket.count / peak) * (HISTOGRAM_BASE - 6);
            return (
              <rect
                key={bucket.from_seconds}
                x={`${(bucket.from_seconds / axisSeconds) * 100}%`}
                y={HISTOGRAM_BASE - height}
                width={`${Math.max(0.4, bucketPct - 0.5)}%`}
                height={height}
                className="fill-primary/70"
              />
            );
          })}
          <line
            x1={`${medianPct}%`}
            y1="0"
            x2={`${medianPct}%`}
            y2={HISTOGRAM_BASE}
            className="stroke-warning"
            strokeWidth="1.5"
            strokeDasharray="3 3"
          />
          <text
            x="0"
            y={HISTOGRAM_HEIGHT - 3}
            className="fill-fg-3 font-mono text-meta tracking-normal"
          >
            0 s
          </text>
          <text
            x="100%"
            y={HISTOGRAM_HEIGHT - 3}
            textAnchor="end"
            className="fill-fg-3 font-mono text-meta tracking-normal"
          >
            {axisSeconds} s
          </text>
        </svg>
      )}
    </figure>
  );
}
