'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { gappedPoints, linePath, linearScale, mean, niceTicks, segmentsOf, smoothPath } from '@/lib/analysis/chart';
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
  /** One value per category, in the categories' order; null leaves a gap. */
  readonly values: readonly (number | null)[];
  /** A colour token such as `--fg-1` or `--chart-2`; the first series defaults to `--fg-1`. */
  readonly color?: string;
  /** Drawn thicker, such as the team's own line among its players. */
  readonly emphasis?: boolean;
  /** Starts hidden; the legend turns it on. */
  readonly hidden?: boolean;
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

const PAD = { top: 10, right: 104, bottom: 34, left: 40 };
const FALLBACK_COLORS = ['--fg-1', '--chart-1', '--chart-2', '--chart-3', '--chart-6', '--chart-7'];
const MAX_X_LABELS = 12;
// The first and last points open their tooltip inwards so it never leaves the chart.
const TOOLTIP_SHIFT = { start: 'translate-x-0', middle: '-translate-x-1/2', end: '-translate-x-full' } as const;
const AVERAGE_ID = '__average';
const REFERENCE_ID = '__reference';

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

function present(values: readonly (number | null)[]): number[] {
  return values.filter((value): value is number => value !== null);
}

function LegendButton({
  label,
  on,
  onToggle,
  swatch,
}: {
  label: string;
  on: boolean;
  onToggle: () => void;
  swatch: ReactNode;
}): ReactNode {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onToggle}
      className={cn(
        'inline-flex h-6 items-center gap-1.5 rounded-sm px-1.5 text-meta outline-none transition-[color,opacity]',
        'hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50',
        on ? 'text-fg-1' : 'text-fg-3 opacity-60',
      )}
    >
      {swatch}
      {label}
    </button>
  );
}

/**
 * One or more series over categories, Skybox style: smooth lines, dashed
 * average and reference labelled at the right edge, and a legend whose entries
 * switch each line on and off. A tooltip follows hover or the arrow keys, and a
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
  const [hidden, setHidden] = useState<ReadonlySet<string>>(() => new Set(series.filter((entry) => entry.hidden).map((entry) => entry.id)));
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
  const toggle = (id: string): void =>
    setHidden((current) => {
      const next = new Set(current);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  const visible = series.map((entry, index) => ({ entry, index })).filter(({ entry }) => !hidden.has(entry.id));
  const average = showAverage && !hidden.has(AVERAGE_ID) ? mean(present(first.values)) : null;
  const shownReference = reference && !hidden.has(REFERENCE_ID) ? reference : null;
  const all = visible.flatMap(({ entry }) => present(entry.values));
  const extremes = [...(all.length > 0 ? all : present(first.values)), ...(average === null ? [] : [average]), ...(shownReference ? [shownReference.value] : [])];
  const { ticks, domain } = niceTicks(Math.min(...extremes), Math.max(...extremes), 4);
  const plotW = WIDTH - PAD.left - PAD.right;
  const plotH = height - PAD.top - PAD.bottom;
  const y = linearScale(domain, [plotH, 0]);
  const frame = { domain, width: plotW, height: plotH };
  const xAt = (index: number): number => (categories.length === 1 ? plotW / 2 : (index * plotW) / (categories.length - 1));
  const step = categories.length > 1 ? plotW / (categories.length - 1) : plotW;
  // The value line under the labels only reads clearly when one series is on.
  const single = visible.length === 1 ? visible[0]?.entry : undefined;
  const shown = active === null ? null : categories[active];
  const path = smooth ? smoothPath : linePath;
  const legend = series.length > 1 || showAverage || reference !== undefined;

  function onKey(event: KeyboardEvent<SVGSVGElement>): void {
    if (event.key === 'ArrowRight') setActive((index) => Math.min(categories.length - 1, index === null ? 0 : index + 1));
    else if (event.key === 'ArrowLeft') setActive((index) => Math.max(0, index === null ? categories.length - 1 : index - 1));
    else if (event.key === 'Escape') setActive(null);
    else return;
    event.preventDefault();
  }

  return (
    <figure ref={frameRef} className={cn('relative flex flex-col gap-1.5', className)}>
      {toolbar || legend ? (
        <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1.5">
          {toolbar ? <div className="flex flex-wrap items-center gap-1.5">{toolbar}</div> : <span />}
          {legend ? (
            <figcaption aria-label="Líneas del gráfico" className="flex flex-wrap items-center gap-0.5">
              {series.map((entry, index) => (
                <LegendButton
                  key={entry.id}
                  label={entry.label}
                  on={!hidden.has(entry.id)}
                  onToggle={() => toggle(entry.id)}
                  swatch={<span aria-hidden className="h-0.5 w-3 rounded-full" style={{ backgroundColor: colorOf(entry, index) }} />}
                />
              ))}
              {showAverage ? (
                <LegendButton
                  label={series.length > 1 ? `Media de ${first.label}` : 'Media'}
                  on={!hidden.has(AVERAGE_ID)}
                  onToggle={() => toggle(AVERAGE_ID)}
                  swatch={<span aria-hidden className="w-3 border-t-2 border-dashed border-fg-3" />}
                />
              ) : null}
              {reference ? (
                <LegendButton
                  label={reference.label}
                  on={!hidden.has(REFERENCE_ID)}
                  onToggle={() => toggle(REFERENCE_ID)}
                  swatch={<span aria-hidden className="w-3 border-t-2 border-dotted border-fg-4" />}
                />
              ) : null}
            </figcaption>
          ) : null}
        </div>
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
          {labelledIndexes(categories.length).map((index) => {
            const value = single?.values[index];
            return (
              <g key={index} className={cn(active === index ? 'fill-fg-1' : 'fill-fg-3')}>
                <text x={xAt(index)} y={plotH + 14} textAnchor={anchorOf(index, categories.length)} className="text-meta">
                  {categories[index]?.label}
                </text>
                <text x={xAt(index)} y={plotH + 28} textAnchor={anchorOf(index, categories.length)} className="fill-fg-2 text-meta font-medium tabular-nums">
                  {value === undefined || value === null ? '' : format(value)}
                </text>
              </g>
            );
          })}
          {shownReference ? (
            <g>
              <line x1={0} x2={plotW} y1={y(shownReference.value)} y2={y(shownReference.value)} className="stroke-fg-4" strokeDasharray="2 4" />
              <text x={plotW + 6} y={y(shownReference.value)} dy="0.32em" className="fill-fg-3 text-meta">
                {shownReference.label} {format(shownReference.value)}
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
          {active === null ? null : <line x1={xAt(active)} x2={xAt(active)} y1={0} y2={plotH} className="stroke-border-strong" />}
          {visible.map(({ entry, index }) => {
            const color = colorOf(entry, index);
            const points = gappedPoints(entry.values, frame);
            const strong = entry.emphasis === true || series.length === 1;
            return (
              <g key={entry.id}>
                {segmentsOf(points).map((run, at) => (
                  <path key={at} d={path(run)} fill="none" stroke={color} strokeWidth={strong ? 2.25 : 1.5} strokeLinejoin="round" />
                ))}
                {points.map((point, at) =>
                  point === null ? null : (
                    <circle
                      key={at}
                      cx={point.x}
                      cy={point.y}
                      r={active === at ? 4 : 2.5}
                      stroke={color}
                      fill={active === at ? color : 'var(--surface-2)'}
                      strokeWidth={1.5}
                    />
                  ),
                )}
              </g>
            );
          })}
          {categories.map((_, index) => (
            <rect
              key={`hit-${index}`}
              x={xAt(index) - step / 2}
              y={0}
              width={step}
              height={plotH}
              fill="transparent"
              onMouseEnter={() => setActive(index)}
            />
          ))}
        </g>
      </svg>
      {shown && active !== null ? (
        <div
          role="status"
          className={cn(
            'pointer-events-none absolute top-10 z-10 flex flex-col rounded-md border border-border bg-popover px-2.5 py-1.5 text-meta text-fg-2 shadow-[var(--elev-3)]',
            TOOLTIP_SHIFT[anchorOf(active, categories.length)],
          )}
          style={{ left: `${((PAD.left + xAt(active)) / WIDTH) * 100}%` }}
        >
          <span className="text-fg-1">{shown.label}</span>
          {visible.map(({ entry, index }) => {
            const value = entry.values[active];
            if (value === undefined || value === null) return null;
            return (
              <span key={entry.id} className="flex items-center justify-between gap-3 tabular-nums">
                {series.length > 1 ? <span style={{ color: colorOf(entry, index) }}>{entry.label}</span> : null}
                <span className="font-medium text-fg-1">{format(value)}</span>
              </span>
            );
          })}
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
              {series.map((entry) => {
                const value = entry.values[index];
                return <td key={entry.id}>{value === undefined || value === null ? 'Sin dato' : format(value)}</td>;
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </figure>
  );
}
