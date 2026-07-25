import { RADAR_LEVELS, TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from './api/tactical.ts';
import type { RadarCalibration, RadarLevel, TacticalFrame, TacticalSample } from './api/tactical.ts';
import { isMultiLevelMap, levelForAltitude } from './tactical-transform.ts';

/**
 * Presentation-only smoothing of the 8 Hz position samples.
 *
 * Everything here exists so a dot glides instead of stepping 125 ms at a time.
 * None of it may ever feed a displayed statistic: an interpolated point is a
 * guess about the space between two facts. Health, the sample flags, and the
 * slot always come from the earlier real sample, so every number on screen is
 * one the demo actually recorded.
 */

/** Where a playhead tick falls between two decoded frames. */
export type FrameCursor = {
  /** Index of the last frame at or before the tick. */
  index: number;
  /** 0 at that frame, approaching 1 at the next one. */
  alpha: number;
};

/**
 * Locates a tick inside a round's frames by binary search. Before the first
 * frame it pins to the first, after the last it pins to the last, both with no
 * interpolation, so a scrub past either edge holds a real sample rather than
 * extrapolating past the data.
 */
export function frameCursor(frames: readonly TacticalFrame[], tick: number): FrameCursor | undefined {
  if (frames.length === 0) return undefined;
  if (tick <= frames[0].tick) return { index: 0, alpha: 0 };
  const last = frames.length - 1;
  if (tick >= frames[last].tick) return { index: last, alpha: 0 };

  let low = 0;
  let high = last;
  while (low < high) {
    const mid = Math.ceil((low + high) / 2);
    if (frames[mid].tick <= tick) low = mid;
    else high = mid - 1;
  }
  const span = frames[low + 1].tick - frames[low].tick;
  const alpha = span > 0 ? (tick - frames[low].tick) / span : 0;
  return { index: low, alpha };
}

/** Normalizes degrees into `[0, 360)`. */
function normalizeDegrees(value: number): number {
  return ((value % 360) + 360) % 360;
}

/**
 * Interpolates a heading along the shortest arc, so a player turning past due
 * west spins 10° the short way instead of 350° the long way.
 */
export function lerpAngleDegrees(from: number, to: number, t: number): number {
  if (!Number.isFinite(from) || !Number.isFinite(to)) return normalizeDegrees(from);
  const delta = ((to - from + 540) % 360) - 180;
  return normalizeDegrees(from + delta * t);
}

function lerp(from: number, to: number, t: number): number {
  return from + (to - from) * t;
}

/** Reports whether a sample's owner was alive at that tick. */
export function isAlive(sample: TacticalSample): boolean {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive);
}

/**
 * The drawable samples for a cursor: positions and yaw eased towards the next
 * frame, everything else taken verbatim from the earlier one.
 *
 * A slot missing from the next frame, or whose alive bit changes between the
 * two, is not eased at all — a body must not slide to where the corpse was
 * recorded, and a slot that stopped being sampled has no destination.
 */
export function interpolatedSamples(
  frames: readonly TacticalFrame[],
  cursor: FrameCursor,
): TacticalSample[] {
  const current = frames[cursor.index];
  if (current === undefined) return [];
  const next = frames[cursor.index + 1];
  if (next === undefined || cursor.alpha <= 0) return current.samples;

  return current.samples.map((sample) => {
    const ahead = next.samples.find((candidate) => candidate.slot === sample.slot);
    if (ahead === undefined || isAlive(ahead) !== isAlive(sample)) return sample;
    return {
      slot: sample.slot,
      x: lerp(sample.x, ahead.x, cursor.alpha),
      y: lerp(sample.y, ahead.y, cursor.alpha),
      z: lerp(sample.z, ahead.z, cursor.alpha),
      yaw: lerpAngleDegrees(sample.yaw, ahead.yaw, cursor.alpha),
      health: sample.health,
      flags: sample.flags,
    };
  });
}

/** A point of a motion trail, oldest first. */
export type TrailPoint = { x: number; y: number };

/**
 * The last `seconds` of movement per slot, oldest point first, ending at the
 * cursor's frame. Only living samples contribute, so a trail stops at the moment
 * of death instead of pinning a line to a body for the rest of the round.
 */
export function sampleTrails(
  frames: readonly TacticalFrame[],
  cursor: FrameCursor,
  tickrate: number,
  seconds: number,
): Map<number, TrailPoint[]> {
  const trails = new Map<number, TrailPoint[]>();
  const current = frames[cursor.index];
  if (current === undefined) return trails;
  const oldestTick = current.tick - seconds * tickrate;

  for (let i = cursor.index; i >= 0; i -= 1) {
    const frame = frames[i];
    if (frame.tick < oldestTick) break;
    for (const sample of frame.samples) {
      if (!isAlive(sample)) continue;
      const points = trails.get(sample.slot);
      if (points === undefined) trails.set(sample.slot, [{ x: sample.x, y: sample.y }]);
      else points.unshift({ x: sample.x, y: sample.y });
    }
  }
  return trails;
}

/**
 * The vertical section to draw at full strength: the one most live players are
 * on. A single-level map always answers `default`, and a tie keeps `default` so
 * the view never flickers between layers on an even split.
 */
export function dominantLevel(
  calibration: RadarCalibration,
  samples: readonly TacticalSample[],
): RadarLevel {
  if (!isMultiLevelMap(calibration)) return RADAR_LEVELS.default;
  let lower = 0;
  let upper = 0;
  for (const sample of samples) {
    if (!isAlive(sample)) continue;
    if (levelForAltitude(calibration, sample.z) === RADAR_LEVELS.lower) lower += 1;
    else upper += 1;
  }
  return lower > upper ? RADAR_LEVELS.lower : RADAR_LEVELS.default;
}
