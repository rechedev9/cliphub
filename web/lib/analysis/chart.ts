/**
 * The arithmetic of the hand-drawn SVG charts: scales, axis ticks and paths.
 * Coordinates are SVG user units with y growing downward.
 */

export type ChartPoint = { x: number; y: number };

/** A linear map from a domain onto a range; a flat domain maps to the range's middle. */
export function linearScale(domain: readonly [number, number], range: readonly [number, number]): (value: number) => number {
  const [d0, d1] = domain;
  const [r0, r1] = range;
  if (d1 === d0) return () => (r0 + r1) / 2;
  return (value) => r0 + ((value - d0) / (d1 - d0)) * (r1 - r0);
}

function niceStep(rough: number): number {
  const power = 10 ** Math.floor(Math.log10(rough));
  const fraction = rough / power;
  if (fraction <= 1) return power;
  if (fraction <= 2) return 2 * power;
  if (fraction <= 5) return 5 * power;
  return 10 * power;
}

/**
 * About `count` round tick values covering [min, max], and the domain they
 * span. A flat series still gets a visible band around its value.
 */
export function niceTicks(min: number, max: number, count = 4): { ticks: number[]; domain: [number, number] } {
  if (!Number.isFinite(min) || !Number.isFinite(max)) return { ticks: [0, 1], domain: [0, 1] };
  let lo = Math.min(min, max);
  let hi = Math.max(min, max);
  if (hi === lo) {
    const pad = lo === 0 ? 1 : Math.abs(lo) * 0.1;
    lo -= pad;
    hi += pad;
  }
  const step = niceStep((hi - lo) / Math.max(1, count - 1));
  const start = Math.floor(lo / step) * step;
  const end = Math.ceil(hi / step) * step;
  const ticks: number[] = [];
  // Rounding keeps 0.30000000000000004 out of the labels.
  for (let tick = start; tick <= end + step / 2; tick += step) ticks.push(Math.round(tick / step) * step);
  return { ticks, domain: [start, end] };
}

/** Points of a series spread evenly across `width`, scaled into `height` by `domain`. */
export function seriesPoints(values: readonly number[], domain: readonly [number, number], width: number, height: number): ChartPoint[] {
  const x = linearScale([0, Math.max(1, values.length - 1)], [0, width]);
  const y = linearScale(domain, [height, 0]);
  if (values.length === 1) return [{ x: width / 2, y: y(values[0] ?? 0) }];
  return values.map((value, index) => ({ x: x(index), y: y(value) }));
}

/** An SVG path through the points, rounded to a tenth of a unit. */
export function linePath(points: readonly ChartPoint[]): string {
  const r = (n: number): number => Math.round(n * 10) / 10;
  return points.map((point, index) => `${index === 0 ? 'M' : 'L'}${r(point.x)} ${r(point.y)}`).join(' ');
}

/**
 * A smooth SVG path through the points (monotone cubic, Fritsch-Carlson): the
 * curve never overshoots a point, so a peak on the chart is a real peak.
 */
export function smoothPath(points: readonly ChartPoint[]): string {
  const n = points.length;
  if (n < 3) return linePath(points);
  const r = (v: number): number => Math.round(v * 10) / 10;
  const dx: number[] = [];
  const slope: number[] = [];
  for (let i = 0; i < n - 1; i += 1) {
    const a = points[i] ?? { x: 0, y: 0 };
    const b = points[i + 1] ?? a;
    dx.push(b.x - a.x);
    slope.push(b.x === a.x ? 0 : (b.y - a.y) / (b.x - a.x));
  }
  const tangent: number[] = [slope[0] ?? 0];
  for (let i = 1; i < n - 1; i += 1) {
    const s0 = slope[i - 1] ?? 0;
    const s1 = slope[i] ?? 0;
    tangent.push(s0 * s1 <= 0 ? 0 : (3 * (dx[i - 1] ?? 0) + 3 * (dx[i] ?? 0)) / ((2 * (dx[i] ?? 0) + (dx[i - 1] ?? 0)) / s0 + ((dx[i] ?? 0) + 2 * (dx[i - 1] ?? 0)) / s1));
  }
  tangent.push(slope[n - 2] ?? 0);
  const first = points[0] ?? { x: 0, y: 0 };
  let d = `M${r(first.x)} ${r(first.y)}`;
  for (let i = 0; i < n - 1; i += 1) {
    const a = points[i] ?? first;
    const b = points[i + 1] ?? a;
    const h = (dx[i] ?? 0) / 3;
    d += ` C${r(a.x + h)} ${r(a.y + (tangent[i] ?? 0) * h)} ${r(b.x - h)} ${r(b.y - (tangent[i + 1] ?? 0) * h)} ${r(b.x)} ${r(b.y)}`;
  }
  return d;
}

/** The arithmetic mean, or null for an empty series. */
export function mean(values: readonly number[]): number | null {
  if (values.length === 0) return null;
  return values.reduce((sum, value) => sum + value, 0) / values.length;
}
