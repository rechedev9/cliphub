import type { WinPoint } from '../api/metrics.ts';

/** One step of the CT win probability placed on the replay bar. */
export type WinStripPoint = { fraction: number; probability: number };

/**
 * Places the win-probability steps on the bar. A step's `seconds` counts from
 * freeze end, so `offsetSeconds` is the freeze band on a round bar and 0 on the
 * shared clock of superposed rounds.
 */
export function winStripPoints(points: readonly WinPoint[], offsetSeconds: number, durationSeconds: number): WinStripPoint[] {
  if (!(durationSeconds > 0)) return [];
  return points
    .map((point) => ({
      fraction: Math.min(1, Math.max(0, (offsetSeconds + point.seconds) / durationSeconds)),
      probability: Math.min(1, Math.max(0, point.probability_ct)),
    }))
    .sort((a, b) => a.fraction - b.fraction);
}

/** A step path over a `width` × `height` box: CT at the top, T at the bottom. */
export function winStripPath(points: readonly WinStripPoint[], width: number, height: number): string {
  if (points.length === 0) return '';
  const y = (probability: number): number => Math.round((1 - probability) * height * 10) / 10;
  const x = (fraction: number): number => Math.round(fraction * width * 10) / 10;
  let path = `M${x(points[0].fraction)} ${y(points[0].probability)}`;
  for (let i = 1; i < points.length; i++) {
    path += ` H${x(points[i].fraction)} V${y(points[i].probability)}`;
  }
  return `${path} H${width}`;
}

/** The CT win probability at a moment `secondsSinceFreeze` into the live round, or null before the first step. */
export function winProbabilityAt(points: readonly WinPoint[], secondsSinceFreeze: number): number | null {
  let value: number | null = null;
  for (const point of points) {
    if (point.seconds > secondsSinceFreeze) break;
    value = point.probability_ct;
  }
  return value;
}
