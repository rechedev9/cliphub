import type { ReactNode } from 'react';
import type { TimeBin } from '@/lib/analysis/grenade-patterns';
import { elapsedLabel } from '@/lib/tactical-spatial';

const TICKS = [0, 30, 60, 90];

/** When in the round the throws in view went off, in bins of a few seconds. */
export function ThrowTimeHistogram({ bins }: { bins: readonly TimeBin[] }): ReactNode {
  const max = bins.reduce((top, bin) => Math.max(top, bin.count), 0);
  const peak = bins.find((bin) => bin.count === max);
  const end = bins.at(-1)?.end ?? 1;
  const summary =
    peak === undefined || max === 0
      ? 'Sin lanzamientos'
      : `Más lanzamientos entre ${elapsedLabel(peak.start)} y ${elapsedLabel(peak.end)}: ${max}`;

  return (
    <figure className="flex flex-col gap-1.5">
      <figcaption className="flex items-baseline justify-between gap-2 text-body-sm text-fg-2">
        <span className="font-medium text-fg-1">Momento del lanzamiento</span>
        <span className="text-meta text-fg-3">{summary}</span>
      </figcaption>
      <div role="img" aria-label={summary} className="flex h-20 items-end gap-px border-b border-border">
        {bins.map((bin, index) => (
          <div
            key={bin.start}
            title={`${elapsedLabel(bin.start)} a ${elapsedLabel(bin.end)}${index === bins.length - 1 ? ' o más' : ''}: ${bin.count}`}
            className="flex-1 rounded-t-xs bg-fg-3"
            style={{ height: max === 0 ? 0 : `${(bin.count / max) * 100}%` }}
          />
        ))}
      </div>
      <div aria-hidden className="relative h-4 text-meta text-fg-3 tabular-nums">
        {TICKS.filter((tick) => tick < end).map((tick) => (
          <span key={tick} className="absolute" style={{ left: `${(tick / end) * 100}%` }}>
            {elapsedLabel(tick)}
          </span>
        ))}
      </div>
    </figure>
  );
}
