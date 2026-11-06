/**
 * How a number reads against a reference: the comparison of a stat card and
 * the tint of a heat table cell. Pure, so the colour of a cell is testable.
 */

export type DeltaTone = 'better' | 'worse' | 'even';

/** The difference to a reference and whether it is good news. Differences under `epsilon` read as even. */
export function compareDelta(
  value: number,
  reference: number,
  options: { higherIsBetter: boolean; epsilon?: number },
): { delta: number; tone: DeltaTone } {
  const delta = value - reference;
  const epsilon = options.epsilon ?? 1e-9;
  if (!Number.isFinite(delta) || Math.abs(delta) < epsilon) return { delta: Number.isFinite(delta) ? delta : 0, tone: 'even' };
  const better = options.higherIsBetter ? delta > 0 : delta < 0;
  return { delta, tone: better ? 'better' : 'worse' };
}

/** ▲ when the value is above the reference, ▼ below, nothing when even. The colour, not the glyph, says if it is good. */
export function deltaArrow(delta: number, epsilon = 1e-9): '▲' | '▼' | '' {
  if (!Number.isFinite(delta) || Math.abs(delta) < epsilon) return '';
  return delta > 0 ? '▲' : '▼';
}

/** Tint steps either side of the baseline. */
export const HEAT_STEPS = 3;

/**
 * A heat step from -HEAT_STEPS (much worse) to HEAT_STEPS (much better).
 * `spread` is the difference that earns the strongest tint, such as 15 points
 * of win rate; anything inside a sixth of it stays untinted.
 */
export function heatLevel(value: number, baseline: number, spread: number, higherIsBetter: boolean): number {
  if (!Number.isFinite(value) || !Number.isFinite(baseline) || !(spread > 0)) return 0;
  const signed = ((value - baseline) / spread) * (higherIsBetter ? 1 : -1);
  const level = Math.round(signed * HEAT_STEPS);
  return Math.max(-HEAT_STEPS, Math.min(HEAT_STEPS, level));
}

/** The weighted mean of `value` over rows weighed by `weight`; null without weight. */
export function weightedMean<T>(rows: readonly T[], value: (row: T) => number, weight: (row: T) => number): number | null {
  let sum = 0;
  let total = 0;
  for (const row of rows) {
    const w = weight(row);
    if (!(w > 0)) continue;
    sum += value(row) * w;
    total += w;
  }
  return total === 0 ? null : sum / total;
}
