import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '../../api/tactical.ts';
import type { TacticalFrame } from '../../api/tactical.ts';
import type { FrameCursor } from '../../tactical-replay.ts';
import type { Vec3 } from './orbit.ts';

/**
 * `sampleTrails` with heights: the last `seconds` of living movement per slot,
 * oldest first, ending at the cursor's frame, so a trail climbs the stairs too.
 */
export function sampleTrails3d(
  frames: readonly TacticalFrame[],
  cursor: FrameCursor,
  tickrate: number,
  seconds: number,
): Map<number, Vec3[]> {
  const trails = new Map<number, Vec3[]>();
  const current = frames[cursor.index];
  if (current === undefined) return trails;
  const oldestTick = current.tick - seconds * tickrate;
  for (let i = cursor.index; i >= 0; i -= 1) {
    const frame = frames[i];
    if (frame.tick < oldestTick) break;
    for (const sample of frame.samples) {
      if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) continue;
      const point = { x: sample.x, y: sample.y, z: sample.z };
      const points = trails.get(sample.slot);
      if (points === undefined) trails.set(sample.slot, [point]);
      else points.push(point);
    }
  }
  for (const points of trails.values()) points.reverse();
  return trails;
}

/** Recomputes only when the cursor reaches another frame. */
export function createTrail3dReader(
  frames: readonly TacticalFrame[],
  tickrate: number,
  seconds: number,
): (cursor: FrameCursor) => ReadonlyMap<number, readonly Vec3[]> {
  let index: number | undefined;
  let trails: ReadonlyMap<number, readonly Vec3[]> = new Map();
  return (cursor) => {
    if (cursor.index !== index) {
      trails = sampleTrails3d(frames, cursor, tickrate, seconds);
      index = cursor.index;
    }
    return trails;
  };
}
