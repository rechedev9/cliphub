'use client';

import { useState, type KeyboardEvent, type ReactNode } from 'react';
import { linePath, linearScale, mean, niceTicks, seriesPoints } from '@/lib/analysis/chart';
import { cn } from '@/lib/utils';

export type LineChartPoint = {
  /** Category under the point, such as "Mirage, 3 nov". */
  readonly label: string;
  readonly value: number;
  /** Extra line in the tooltip, such as the sample size. */
  readonly detail?: string;
};

export type LineChartProps = {
  /** Accessible name of the chart and caption of its data table. */
  label: string;
  /** Points in reading order, oldest first. */
  points: readonly LineChartPoint[];
  format: (value: number) => string;
  /** Draws the series mean as a dashed line labelled "Media". */
  showAverage?: boolean;
  /** A reference drawn as a second dashed line, such as the library average. */
  reference?: { readonly value: number; readonly label: string };
  height?: number;
  className?: string;
};

const WIDTH = 640;

function anchorOf(index: number, count: number): 'start' | 'middle' | 'end' {
  if (index === 0) return 'start';
  return index === count - 1 ? 'end' : 'middle';
}
const PAD = { top: 12, right: 12, bottom: 24, left: 44 };

/**
 * A single series over categories with a y axis, an optional average line and
 * a tooltip on hover or arrow keys. A screen reader gets the same numbers as a
 * table.
 */
export function LineChart({ label, points, format, showAverage = false, reference, height = 180, className }: LineChartProps): ReactNode {
  const [active, setActive] = useState<number | null>(null);
  if (points.length === 0) return null;
  const values = points.map((point) => point.value);
  const average = showAverage ? mean(values) : null;
  const extremes = [...values, ...(average === null ? [] : [average]), ...(reference ? [reference.value] : [])];
  const { ticks, domain } = niceTicks(Math.min(...extremes), Math.max(...extremes), 4);
  const plotW = WIDTH - PAD.left - PAD.right;
  const plotH = height - PAD.top - PAD.bottom;
  const coords = seriesPoints(values, domain, plotW, plotH);
  const y = linearScale(domain, [plotH, 0]);
  const step = points.length > 1 ? plotW / (points.length - 1) : plotW;
  const shown = active === null ? null : points[active];
  const shownAt = active === null ? null : coords[active];

  function onKey(event: KeyboardEvent<SVGSVGElement>): void {
    if (event.key === 'ArrowRight') setActive((index) => Math.min(points.length - 1, index === null ? 0 : index + 1));
    else if (event.key === 'ArrowLeft') setActive((index) => Math.max(0, index === null ? points.length - 1 : index - 1));
    else if (event.key === 'Escape') setActive(null);
    else return;
    event.preventDefault();
  }

  const xLabels = points.length <= 2 ? [0, points.length - 1] : [0, Math.floor((points.length - 1) / 2), points.length - 1];

  return (
    <figure className={cn('relative flex flex-col gap-2', className)}>
      <svg
        viewBox={`0 0 ${WIDTH} ${height}`}
        className="h-auto max-h-60 w-full rounded-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        role="img"
        aria-label={`${label}. Usa las flechas para recorrer los puntos.`}
        tabIndex={0}
        onKeyDown={onKey}
        onBlur={() => setActive(null)}
        onMouseLeave={() => setActive(null)}
      >
        <g transform={`translate(${PAD.left} ${PAD.top})`}>
          {ticks.map((tick) => (
            <g key={tick}>
              <line x1={0} x2={plotW} y1={y(tick)} y2={y(tick)} className="stroke-border-subtle" />
              <text x={-8} y={y(tick)} dy="0.32em" textAnchor="end" className="fill-fg-3 text-meta tabular-nums">
                {format(tick)}
              </text>
            </g>
          ))}
          {[...new Set(xLabels)].map((index) => (
            <text
              key={index}
              x={coords[index]?.x ?? 0}
              y={plotH + 18}
              textAnchor={anchorOf(index, points.length)}
              className="fill-fg-3 text-meta"
            >
              {points[index]?.label}
            </text>
          ))}
          {reference ? (
            <g>
              <line x1={0} x2={plotW} y1={y(reference.value)} y2={y(reference.value)} className="stroke-fg-4" strokeDasharray="2 4" />
              <text x={plotW} y={y(reference.value) - 4} textAnchor="end" className="fill-fg-3 text-meta">
                {reference.label} {format(reference.value)}
              </text>
            </g>
          ) : null}
          {average !== null ? (
            <g>
              <line x1={0} x2={plotW} y1={y(average)} y2={y(average)} className="stroke-fg-3" strokeDasharray="5 4" />
              <text x={4} y={y(average) - 4} className="fill-fg-2 text-meta">
                Media {format(average)}
              </text>
            </g>
          ) : null}
          <path d={linePath(coords)} fill="none" className="stroke-fg-1" strokeWidth={2} strokeLinejoin="round" />
          {coords.map((point, index) => (
            <circle
              key={index}
              cx={point.x}
              cy={point.y}
              r={active === index ? 4.5 : 3}
              className={cn('stroke-fg-1', active === index ? 'fill-fg-1' : 'fill-surface-2')}
              strokeWidth={1.5}
            />
          ))}
          {coords.map((point, index) => (
            <rect
              key={`hit-${index}`}
              x={point.x - step / 2}
              y={0}
              width={step}
              height={plotH}
              fill="transparent"
              onMouseEnter={() => setActive(index)}
            />
          ))}
        </g>
      </svg>
      {shown && shownAt ? (
        <div
          role="status"
          className="pointer-events-none absolute top-0 z-10 flex -translate-x-1/2 flex-col rounded-md border border-border bg-popover px-2.5 py-1.5 text-meta text-fg-2 shadow-[var(--elev-3)]"
          style={{ left: `${((PAD.left + shownAt.x) / WIDTH) * 100}%` }}
        >
          <span className="text-fg-1">{shown.label}</span>
          <span className="font-medium text-fg-1 tabular-nums">{format(shown.value)}</span>
          {shown.detail ? <span>{shown.detail}</span> : null}
        </div>
      ) : null}
      <table className="sr-only">
        <caption>{label}</caption>
        <tbody>
          {points.map((point, index) => (
            <tr key={index}>
              <th scope="row">{point.label}</th>
              <td>{format(point.value)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
