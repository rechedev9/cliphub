import { RADAR_LEVELS, TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '../api/tactical.ts';
import type { RadarCalibration, RadarLevel, TacticalFrame } from '../api/tactical.ts';
import { isMultiLevelMap, levelForAltitude } from '../tactical-transform.ts';

// A player's floor over time: a dead band round the altitude cut stops ramp and stair flicker,
// and standing still drops the band so a camper never stays on the wrong block.

/** World units either side of the altitude cut that keep the floor a player was on. */
export const FLOOR_BAND = 48;
/** Standing still for this many ticks (2 s at 64 tick), within `STILL_RISE`, drops the band. */
export const STILL_TICKS = 128;
export const STILL_RISE = 6;

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
 * The frames in which a living player stands still: their height stays within
 * `STILL_RISE` for at least `STILL_TICKS`. Frames where the slot is missing or dead break a run.
 */
function stillFrames(options: { frames: readonly TacticalFrame[]; heights: Float64Array; living: Uint8Array }): Uint8Array {
  const { frames, heights, living } = options;
  const still = new Uint8Array(frames.length);
  let start = 0;
  while (start < frames.length) {
    if (living[start] === 0) {
      start += 1;
      continue;
    }
    let low = heights[start];
    let high = low;
    let end = start;
    while (end + 1 < frames.length && living[end + 1] === 1 && Math.max(high, heights[end + 1]) - Math.min(low, heights[end + 1]) <= STILL_RISE) {
      end += 1;
      low = Math.min(low, heights[end]);
      high = Math.max(high, heights[end]);
    }
    if (frames[end].tick - frames[start].tick >= STILL_TICKS) still.fill(1, start, end + 1);
    start = end + 1;
  }
  return still;
}

/**
 * Walks a round's frames once, in order, and settles every player's floor with
 * the dead band, so the answer for a frame never depends on how the playhead got there.
 */
export function createFloorTimeline(calibration: RadarCalibration, frames: readonly TacticalFrame[]): FloorTimeline {
  const bySlot = new Map<number, Uint8Array>();
  if (isMultiLevelMap(calibration)) {
    const heights = new Map<number, { z: Float64Array; living: Uint8Array }>();
    frames.forEach((frame, index) => {
      for (const sample of frame.samples) {
        let track = heights.get(sample.slot);
        if (track === undefined) {
          track = { z: new Float64Array(frames.length).fill(Number.NaN), living: new Uint8Array(frames.length) };
          heights.set(sample.slot, track);
        }
        track.z[index] = sample.z;
        track.living[index] = hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive) ? 1 : 0;
      }
    });
    for (const [slot, track] of heights) {
      const still = stillFrames({ frames, heights: track.z, living: track.living });
      const levels = new Uint8Array(frames.length);
      let previous: RadarLevel | undefined;
      let held = 0;
      for (let index = 0; index < frames.length; index += 1) {
        const z = track.z[index];
        // A frame without the slot keeps the floor it was on.
        if (Number.isNaN(z)) {
          levels[index] = held;
          continue;
        }
        const level = still[index] === 1 ? levelForAltitude(calibration, z) : assignFloor({ calibration, z, previous });
        previous = level;
        held = level === RADAR_LEVELS.lower ? LOWER : 2;
        levels[index] = held;
      }
      bySlot.set(slot, levels);
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
