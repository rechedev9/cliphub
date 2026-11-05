import type { ReactNode } from 'react';
import { linePath, niceTicks, seriesPoints } from '@/lib/analysis/chart';
import { cn } from '@/lib/utils';

export type SparklineProps = {
  /** The series, oldest first. Fewer than two values draw nothing. */
  values: readonly number[];
  /** What the line shows, read by screen readers with its first and last value. */
  label: string;
  format?: (value: number) => string;
  width?: number;
  height?: number;
  className?: string;
};

/** A small trend line with its latest point marked. No axes: the number beside it carries the value. */
export function Sparkline({ values, label, format = String, width = 96, height = 28, className }: SparklineProps): ReactNode {
  if (values.length < 2) return null;
  const { domain } = niceTicks(Math.min(...values), Math.max(...values), 2);
  const points = seriesPoints(values, domain, width, height);
  const last = points[points.length - 1];
  const first = values[0] ?? 0;
  const latest = values[values.length - 1] ?? 0;
  return (
    <svg
      viewBox={`-2 -2 ${width + 4} ${height + 4}`}
      width={width}
      height={height}
      role="img"
      aria-label={`${label}: de ${format(first)} a ${format(latest)}`}
      className={cn('shrink-0 overflow-visible text-fg-3', className)}
    >
      <path d={linePath(points)} fill="none" stroke="currentColor" strokeWidth={1.5} strokeLinejoin="round" strokeLinecap="round" />
      {last ? <circle cx={last.x} cy={last.y} r={2.5} className="fill-fg-1" /> : null}
    </svg>
  );
}
