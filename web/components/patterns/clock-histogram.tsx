'use client';

import type { ReactNode } from 'react';
import { bucketLabel, type Bucket } from '@/lib/analysis/pattern-canvas';
import { ROUND_CLOCK_SECONDS, roundClock } from '@/lib/replay/tramo';
import { cn } from '@/lib/utils';

/** One colour of the stack: a player or a grenade type. */
export type HistogramKey = { key: string; label: string; token: string };

/**
 * When the counted moments happen over the round clock, a bar per 10 seconds and each bar stacked by
 * player or by grenade type. A click on a bar asks for that bucket as the time window.
 */
export function ClockHistogram({
  buckets,
  tallest,
  keys,
  noun,
  label,
  onPick,
}: {
  buckets: readonly Bucket[];
  tallest: number;
  keys: readonly HistogramKey[];
  noun: { one: string; many: string };
  label: string;
  onPick?: (bucket: Bucket) => void;
}): ReactNode {
  return (
    <figure className="flex flex-col gap-1">
      <div className="flex gap-1.5">
        <div aria-hidden className="flex h-24 flex-col justify-between text-meta tabular-nums text-fg-3">
          <span>{tallest}</span>
          <span>0</span>
        </div>
        <div role="img" aria-label={label} className="flex h-24 min-w-0 flex-1 items-end gap-0.5 border-b border-l border-border-strong px-0.5">
          {buckets.map((bucket) => {
            const title = `${bucketLabel(bucket)}: ${bucket.total} ${bucket.total === 1 ? noun.one : noun.many}`;
            return (
              <button
                key={bucket.start}
                type="button"
                title={title}
                aria-label={title}
                disabled={onPick === undefined || bucket.total === 0}
                onClick={() => onPick?.(bucket)}
                className="flex h-full min-w-0 flex-1 flex-col-reverse justify-start outline-none focus-visible:ring-2 focus-visible:ring-ring/60 enabled:cursor-pointer"
              >
                <span className="flex flex-col-reverse overflow-hidden rounded-t-xs" style={{ height: tallest === 0 ? 0 : `${(bucket.total / tallest) * 100}%` }}>
                  {keys.map((entry) => {
                    const count = bucket.byKey.get(entry.key) ?? 0;
                    return count === 0 ? null : <span key={entry.key} style={{ height: `${(count / bucket.total) * 100}%`, background: `var(${entry.token})` }} />;
                  })}
                </span>
              </button>
            );
          })}
        </div>
      </div>
      <div aria-hidden className="ml-5 flex gap-0.5 px-0.5">
        {buckets.map((bucket, index) => (
          <span key={bucket.start} className={cn('min-w-0 flex-1 text-center text-[9px] tabular-nums text-fg-3', index % 2 === 1 && 'invisible')}>
            {roundClock(bucket.start)}
          </span>
        ))}
      </div>
      <figcaption className="flex flex-wrap gap-x-3 gap-y-0.5 pl-5 text-meta text-fg-3">
        {keys.map((entry) => (
          <span key={entry.key} className="inline-flex items-center gap-1">
            <span aria-hidden className="size-2 rounded-full" style={{ background: `var(${entry.token})` }} />
            {entry.label}
          </span>
        ))}
      </figcaption>
    </figure>
  );
}

/** Each moment as a stripe on the time bar, coloured like the histogram: the barcode of the Buscador. */
export function MomentBarcode({ moments }: { moments: readonly { seconds: number; token: string }[] }): ReactNode {
  return (
    <div aria-hidden className="absolute inset-0">
      {moments.map((moment, index) => (
        <span
          key={index}
          className="absolute inset-y-1 w-px opacity-80"
          style={{ left: `${Math.min(100, Math.max(0, (moment.seconds / ROUND_CLOCK_SECONDS) * 100))}%`, background: `var(${moment.token})` }}
        />
      ))}
    </div>
  );
}
