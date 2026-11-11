import type { RadarCalibration, RadarLevel } from '../api/tactical.ts';
import { levelForAltitude } from '../tactical-transform.ts';

/** On a two-floor map, marks on the floor the radar is not showing keep this strength. */
export const OFF_FLOOR_ALPHA = 0.35;

/**
 * The floor a radar shows: its calibration says where the lower floor ends.
 * With both floors on screen the other floor has its own pane, so `offFloorAlpha` 0 hides its marks here.
 */
export type ShownFloor = { calibration: RadarCalibration; level: RadarLevel; offFloorAlpha?: number };

export function onShownFloor(floor: ShownFloor, z: number): boolean {
  return levelForAltitude(floor.calibration, z) === floor.level;
}

/** Whether a mark's label belongs on this radar: only where the mark itself shows, so a hidden floor keeps no names. */
export function labelOnFloor(floor: ShownFloor, z: number): boolean {
  return floorAlpha(floor, z) > 0;
}

/** 1 for a point at height `z` on the shown floor, `OFF_FLOOR_ALPHA` (or the floor's own) for one on the other floor. */
export function floorAlpha(floor: ShownFloor, z: number): number {
  return onShownFloor(floor, z) ? 1 : (floor.offFloorAlpha ?? OFF_FLOOR_ALPHA);
}

/** The same strength for a mark whose floor is already settled (see `lib/replay/floor-assign`), not read off its height. */
export function levelAlpha(floor: { level: RadarLevel; offFloorAlpha?: number }, level: RadarLevel): number {
  return level === floor.level ? 1 : (floor.offFloorAlpha ?? OFF_FLOOR_ALPHA);
}
