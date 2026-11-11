import type { RadarCalibration, RadarLevel } from '../api/tactical.ts';
import { levelAlpha } from '../analysis/floors.ts';
import type { UtilityVisual } from './three/utility-visuals.ts';
import { levelForAltitude, worldToRendered } from '../tactical-transform.ts';

/** Where a grenade's throw point is drawn, in the radar square's CSS pixels; `index` is its place in the round's events. */
export type ThrowMarker = { index: number; x: number; y: number };

/**
 * The throw points the map draws for these grenades: the origin of each arc still
 * on screen, on the thrower's settled floor, the same pane and transform as the arc.
 */
export function throwMarkers(options: {
  visuals: readonly UtilityVisual[];
  calibration: RadarCalibration;
  size: number;
  /** The floor the pane shows. */
  level: RadarLevel;
  /** How much of the other floor shows in this pane; 0 when each floor has its own. */
  offFloorAlpha?: number;
  /** The floor each thrower stood on as the grenade left the hand, by event index. */
  throwLevels?: ReadonlyMap<number, RadarLevel>;
}): ThrowMarker[] {
  const { visuals, calibration, size, level, offFloorAlpha, throwLevels } = options;
  const out: ThrowMarker[] = [];
  for (const visual of visuals) {
    if (visual.arc === null || visual.from === null || visual.arc.fade <= 0) continue;
    const from = throwLevels?.get(visual.index) ?? levelForAltitude(calibration, visual.from.z);
    if (levelAlpha({ level, offFloorAlpha }, from) <= 0) continue;
    const at = worldToRendered(calibration, visual.from.x, visual.from.y, size);
    out.push({ index: visual.index, x: at.x, y: at.y });
  }
  return out;
}

/** The closest marker within `reach` pixels of a point, as the event index it belongs to. */
export function throwAt(options: { markers: readonly ThrowMarker[]; x: number; y: number; reach: number }): number | null {
  const { markers, x, y, reach } = options;
  let best: { index: number; distance: number } | null = null;
  for (const marker of markers) {
    const distance = Math.hypot(marker.x - x, marker.y - y);
    if (distance <= reach && (best === null || distance < best.distance)) best = { index: marker.index, distance };
  }
  return best?.index ?? null;
}
