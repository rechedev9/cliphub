import type { RadarCalibration, RadarLevel } from '../api/tactical.ts';
import { levelForAltitude } from '../tactical-transform.ts';

/** On a two-floor map, marks on the floor the radar is not showing keep this strength. */
export const OFF_FLOOR_ALPHA = 0.35;

/** The floor a radar shows: its calibration says where the lower floor ends. */
export type ShownFloor = { calibration: RadarCalibration; level: RadarLevel };

export function onShownFloor(floor: ShownFloor, z: number): boolean {
  return levelForAltitude(floor.calibration, z) === floor.level;
}

/** 1 for a point at height `z` on the shown floor, `OFF_FLOOR_ALPHA` for one on the other floor. */
export function floorAlpha(floor: ShownFloor, z: number): number {
  return onShownFloor(floor, z) ? 1 : OFF_FLOOR_ALPHA;
}
