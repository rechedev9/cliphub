'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { linePath, linearScale, mean, niceTicks, seriesPoints, smoothPath } from '@/lib/analysis/chart';
import { cn } from '@/lib/utils';

export type LineChartCategory = {
  /** Label under the point, such as "Mirage 3 nov". */
  readonly label: string;
  /** Extra line in the tooltip, such as the sample size. */
  readonly detail?: string;
};

export type LineChartSeries = {
  readonly id: string;
  readonly label: string;
  /** One value per category, in the categories' order. */
  readonly values: readonly number[];
  /** A colour token such as `--fg-1` or `--side-ct`; the first series defaults to `--fg-1`. */
  readonly color?: string;
};

export type LineChartProps = {
  /** Accessible name of the chart and caption of its data table. */
  label: string;
  categories: readonly LineChartCategory[];
  series: readonly LineChartSeries[];
  format: (value: number) => string;
  /** Draws the first series' mean as a dashed line labelled at the right. */
  showAverage?: boolean;
  /** A reference drawn as a second dashed line, such as the library average. */
  reference?: { readonly value: number; readonly label: string };
  /** Curves through the points instead of straight segments. Default true. */
  smooth?: boolean;
  height?: number;
  /** Controls above the plot, such as the metric chips. */
  toolbar?: ReactNode;
  className?: string;
};

const PAD = { top: 10, right: 96, bottom: 34, left: 40 };
const FALLBACK_COLORS = ['--fg-1', '--chart-1', '--chart-2', '--chart-3'];
const MAX_X_LABELS = 12;

function colorOf(series: LineChartSeries, index: number): string {
  return `var(${series.color ?? FALLBACK_COLORS[index % FALLBACK_COLORS.length] ?? '--fg-1'})`;
}

function anchorOf(index: number, count: number): 'start' | 'middle' | 'end' {
  if (count > 1 && index === 0) return 'start';
  return count > 1 && index === count - 1 ? 'end' : 'middle';
}

/** Category indexes that get a label: all of them when they fit, else evenly thinned keeping both ends. */
function labelledIndexes(count: number): number[] {
  if (count <= MAX_X_LABELS) return Array.from({ length: count }, (_, index) => index);
  const step = (count - 1) / (MAX_X_LABELS - 1);
  return [...new Set(Array.from({ length: MAX_X_LABELS }, (_, index) => Math.round(index * step)))];
}

/**
 * One or more series over categories, Skybox style: smooth lines, a dashed
 * average labelled at the right edge, and each category's label with the first
 * series' value under it. A tooltip follows hover or the arrow keys, and a
 * screen reader gets the same numbers as a table.
 */
export function LineChart({
  label,
  categories,
  series,
  format,
  showAverage = false,
  reference,
  smooth = true,
  height = 220,
  toolbar,
  className,
}: LineChartProps): ReactNode {
  const [active, setActive] = useState<number | null>(null);
  const [WIDTH, setWidth] = useState(760);
  const frameRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const frame = frameRef.current;
    if (frame === null) return undefined;
    // The plot is drawn at the frame's own pixel width, so text stays at its type size at any width.
    const observer = new ResizeObserver(([entry]) => setWidth(Math.max(320, Math.round(entry?.contentRect.width ?? 760))));
    observer.observe(frame);
    return () => observer.disconnect();
  }, []);
  const first = series[0];
  if (categories.length === 0 || first === undefined) return null;
  const all = series.flatMap((entry) => [...entry.values]);
  const average = showAverage ? mean(first.values) : null;
  const extremes = [...all, ...(average === null ? [] : [average]), ...(reference ? [reference.value] : [])];
  const { ticks, domain } = niceTicks(Math.min(...extremes), Math.max(...extremes), 4);
  const plotW = WIDTH - PAD.left - PAD.right;
  const plotH = height - PAD.top - PAD.bottom;
  const y = linearScale(domain, [plotH, 0]);
  const lines = series.map((entry) => seriesPoints(entry.values, domain, plotW, plotH));
  const firstLine = lines[0] ?? [];
  const step = categories.length > 1 ? plotW / (categories.length - 1) : plotW;
  const shown = active === null ? null : categories[active];
  const shownAt = active === null ? null : firstLine[active];
  const path = smooth ? smoothPath : linePath;

  function onKey(event: KeyboardEvent<SVGSVGElement>): void {
    if (event.key === 'ArrowRight') setActive((index) => Math.min(categories.length - 1, index === null ? 0 : index + 1));
    else if (event.key === 'ArrowLeft') setActive((index) => Math.max(0, index === null ? categories.length - 1 : index - 1));
    else if (event.key === 'Escape') setActive(null);
    else return;
    event.preventDefault();
  }

  return (
    <figure ref={frameRef} className={cn('relative flex flex-col gap-1.5', className)}>
      {toolbar ? <div className="flex flex-wrap items-center gap-1.5">{toolbar}</div> : null}
      {series.length > 1 ? (
        <figcaption className="flex flex-wrap items-center gap-3 text-meta text-fg-2">
          {series.map((entry, index) => (
            <span key={entry.id} className="inline-flex items-center gap-1.5">
              <span aria-hidden className="h-0.5 w-3 rounded-full" style={{ backgroundColor: colorOf(entry, index) }} />
              {entry.label}
            </span>
          ))}
        </figcaption>
      ) : null}
      <svg
        viewBox={`0 0 ${WIDTH} ${height}`}
        className="block w-full rounded-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
        style={{ height }}
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
              <text x={-6} y={y(tick)} dy="0.32em" textAnchor="end" className="fill-fg-3 text-meta tabular-nums">
                {format(tick)}
              </text>
            </g>
          ))}
          {labelledIndexes(categories.length).map((index) => (
            <g key={index} className={cn(active === index ? 'fill-fg-1' : 'fill-fg-3')}>
              <text x={firstLine[index]?.x ?? 0} y={plotH + 14} textAnchor={anchorOf(index, categories.length)} className="text-meta">
                {categories[index]?.label}
              </text>
              <text x={firstLine[index]?.x ?? 0} y={plotH + 28} textAnchor={anchorOf(index, categories.length)} className="fill-fg-2 text-meta font-medium tabular-nums">
                {first.values[index] === undefined ? '' : format(first.values[index] ?? 0)}
              </text>
            </g>
          ))}
          {reference ? (
            <g>
              <line x1={0} x2={plotW} y1={y(reference.value)} y2={y(reference.value)} className="stroke-fg-4" strokeDasharray="2 4" />
              <text x={plotW + 6} y={y(reference.value)} dy="0.32em" className="fill-fg-3 text-meta">
                {reference.label} {format(reference.value)}
              </text>
            </g>
          ) : null}
          {average !== null ? (
            <g>
              <line x1={0} x2={plotW} y1={y(average)} y2={y(average)} className="stroke-fg-3" strokeDasharray="5 4" />
              <text x={plotW + 6} y={y(average)} dy="0.32em" className="fill-fg-2 text-meta font-medium">
                Media {format(average)}
              </text>
            </g>
          ) : null}
          {active === null || shownAt === undefined || shownAt === null ? null : (
            <line x1={shownAt.x} x2={shownAt.x} y1={0} y2={plotH} className="stroke-border-strong" />
          )}
          {lines.map((points, index) => {
            const entry = series[index];
            if (entry === undefined) return null;
            const color = colorOf(entry, index);
            return (
              <g key={entry.id}>
                <path d={path(points)} fill="none" stroke={color} strokeWidth={index === 0 ? 2 : 1.5} strokeLinejoin="round" />
                {points.map((point, at) => (
                  <circle
                    key={at}
                    cx={point.x}
                    cy={point.y}
                    r={active === at ? 4 : 2.5}
                    stroke={color}
                    fill={active === at ? color : 'var(--surface-2)'}
                    strokeWidth={1.5}
                  />
                ))}
              </g>
            );
          })}
          {firstLine.map((point, index) => (
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
          className="pointer-events-none absolute top-6 z-10 flex -translate-x-1/2 flex-col rounded-md border border-border bg-popover px-2.5 py-1.5 text-meta text-fg-2 shadow-[var(--elev-3)]"
          style={{ left: `${((PAD.left + shownAt.x) / WIDTH) * 100}%` }}
        >
          <span className="text-fg-1">{shown.label}</span>
          {series.map((entry, index) => (
            <span key={entry.id} className="flex items-center justify-between gap-3 tabular-nums">
              {series.length > 1 ? <span style={{ color: colorOf(entry, index) }}>{entry.label}</span> : null}
              <span className="font-medium text-fg-1">{format(entry.values[active ?? 0] ?? 0)}</span>
            </span>
          ))}
          {shown.detail ? <span>{shown.detail}</span> : null}
        </div>
      ) : null}
      <table className="sr-only">
        <caption>{label}</caption>
        <thead>
          <tr>
            <th scope="col">Punto</th>
            {series.map((entry) => (
              <th key={entry.id} scope="col">
                {entry.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {categories.map((category, index) => (
            <tr key={index}>
              <th scope="row">{category.label}</th>
              {series.map((entry) => (
                <td key={entry.id}>{format(entry.values[index] ?? 0)}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
