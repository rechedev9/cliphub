import { RADAR_LEVELS } from '../api/tactical.ts';
import type { RadarCalibration, RadarLevel, TacticalFrame } from '../api/tactical.ts';
import { isMultiLevelMap, levelForAltitude } from '../tactical-transform.ts';

/**
 * Which floor of a two-floor map a player is on, over time. The calibration's
 * altitude is a hard cut, and a ramp or a stair crosses it slowly, jump by
 * jump: a player must get clearly past it before the floor changes, so nobody
 * flickers between panes. Tuned on paper (a jump is about 46 units): check on
 * a real Nuke and Vertigo demo.
 */

/** World units either side of the altitude cut that keep the floor a player was on. */
export const FLOOR_BAND = 48;

/** The floor at height `z`, keeping `previous` while `z` stays inside the dead band around the cut. */
export function assignFloor(options: { calibration: RadarCalibration; z: number; previous?: RadarLevel }): RadarLevel {
  const { calibration, z, previous } = options;
  const cut = calibration.lower_altitude_max;
  if (cut === undefined) return RADAR_LEVELS.default;
  if (previous === undefined) return levelForAltitude(calibration, z);
  if (previous === RADAR_LEVELS.lower) return z > cut + FLOOR_BAND ? RADAR_LEVELS.default : RADAR_LEVELS.lower;
  return z <= cut - FLOOR_BAND ? RADAR_LEVELS.lower : RADAR_LEVELS.default;
}

/** The floor of each player at each frame of one round. */
export type FloorTimeline = {
  /** The floor of `slot` at frame `index`; a slot the round never recorded answers `fallback`. */
  levelAt: (slot: number, index: number, fallback: RadarLevel) => RadarLevel;
};

const LOWER = 1;

/**
 * Walks a round's frames once, in order, and settles every player's floor with
 * the dead band, so the answer for a frame never depends on how the playhead got there.
 */
export function createFloorTimeline(calibration: RadarCalibration, frames: readonly TacticalFrame[]): FloorTimeline {
  const bySlot = new Map<number, Uint8Array>();
  if (isMultiLevelMap(calibration)) {
    const last = new Map<number, RadarLevel>();
    frames.forEach((frame, index) => {
      for (const sample of frame.samples) {
        const level = assignFloor({ calibration, z: sample.z, previous: last.get(sample.slot) });
        last.set(sample.slot, level);
        let levels = bySlot.get(sample.slot);
        if (levels === undefined) {
          levels = new Uint8Array(frames.length);
          bySlot.set(sample.slot, levels);
        }
        // A frame without the slot keeps the floor it was on, filled in below.
        levels[index] = level === RADAR_LEVELS.lower ? LOWER : 2;
      }
    });
    for (const levels of bySlot.values()) {
      let held = 0;
      for (let i = 0; i < levels.length; i += 1) {
        if (levels[i] === 0) levels[i] = held;
        else held = levels[i];
      }
    }
  }
  return {
    levelAt: (slot, index, fallback) => {
      const levels = bySlot.get(slot);
      const value = levels?.[Math.min(Math.max(index, 0), (levels?.length ?? 1) - 1)];
      if (value === undefined || value === 0) return fallback;
      return value === LOWER ? RADAR_LEVELS.lower : RADAR_LEVELS.default;
    },
  };
}

/**
 * The floor of each point of a throw's path: the thrower's until the flight
 * gets down to (or up to) the landing floor's height, so the arc never hops
 * panes mid-air and the effect stays on the landing floor.
 */
export function arcLevels(calibration: RadarCalibration, path: readonly { z: number }[], ends: { from: RadarLevel; landing: RadarLevel }): RadarLevel[] {
  const { from, landing } = ends;
  if (from === landing) return path.map(() => landing);
  const crossing = path.findIndex((point) => levelForAltitude(calibration, point.z) === landing);
  const at = crossing < 0 ? path.length - 1 : crossing;
  return path.map((_, index) => (index < at ? from : landing));
}
