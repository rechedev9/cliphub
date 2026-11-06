import type { TacticalGeometry } from '../api/tactical.ts';
import { cellRadarRect, isCalibrationUsable, worldToRendered } from '../tactical-transform.ts';

export type ZoneShape = {
  readonly zone: string;
  /** The place's cells as one path; empty when the analysis predates per-place cells. */
  readonly path: string;
  /** Where the label sits, in the same units as the silhouette's viewBox. */
  readonly x: number;
  readonly y: number;
};

/**
 * The named zones drawn over a map silhouette of `size` units: each place's
 * cells and its centre of play. Zones the geometry does not name are left out.
 */
export function zoneShapes(geometry: TacticalGeometry, zones: readonly string[], size = 100): ZoneShape[] {
  if (!isCalibrationUsable(geometry.calibration) || !(geometry.cell_size > 0)) return [];
  const r = (n: number): number => Math.round(n * 100) / 100;
  const out: ZoneShape[] = [];
  for (const zone of zones) {
    const callout = geometry.callouts.find((entry) => entry.name === zone);
    if (callout === undefined) continue;
    const parts = (callout.cells ?? []).map(([cx, cy]) => {
      const rect = cellRadarRect(geometry, [cx, cy, 1], size);
      return `M${r(rect.x)} ${r(rect.y)}h${r(rect.width)}v${r(rect.height)}h${r(-rect.width)}z`;
    });
    const point = worldToRendered(geometry.calibration, callout.center[0], callout.center[1], size);
    out.push({ zone, path: parts.join(''), x: r(point.x), y: r(point.y) });
  }
  return out;
}
