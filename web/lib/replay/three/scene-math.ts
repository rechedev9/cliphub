import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '../../api/tactical.ts';
import type { RadarBounds, TacticalFrame, TacticalSample } from '../../api/tactical.ts';
import type { Vec3, WorldBox } from './orbit.ts';

/** CS2 player hull heights, in world units. */
export const STANDING_HEIGHT = 72;
export const DUCKING_HEIGHT = 54;
/** Eye height over the feet, standing and ducking. */
export const STANDING_EYE = 64;
export const DUCKING_EYE = 46;

/** Unit view vector for CS2 angles: yaw counter-clockwise from +X, positive pitch looks down. */
export function viewDirection(yawDegrees: number, pitchDegrees = 0): Vec3 {
  const yaw = (yawDegrees * Math.PI) / 180;
  const pitch = (pitchDegrees * Math.PI) / 180;
  const ground = Math.cos(pitch);
  return { x: Math.cos(yaw) * ground, y: Math.sin(yaw) * ground, z: -Math.sin(pitch) };
}

/** Yaw in degrees from one point to the next, or null when the player stood still. */
export function movementYaw(from: { x: number; y: number }, to: { x: number; y: number }): number | null {
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  if (Math.hypot(dx, dy) < 1) return null;
  return (((Math.atan2(dy, dx) * 180) / Math.PI) % 360 + 360) % 360;
}

/** The yaw to look along: the recorded one, else the movement direction, else +X. */
export function lookYaw(sample: Pick<TacticalSample, 'x' | 'y' | 'yaw'>, previous?: { x: number; y: number }): number {
  if (Number.isFinite(sample.yaw)) return sample.yaw;
  return (previous === undefined ? null : movementYaw(previous, sample)) ?? 0;
}

export function isDucking(sample: Pick<TacticalSample, 'flags'>): boolean {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.ducking);
}

export function hullHeight(sample: Pick<TacticalSample, 'flags'>): number {
  return isDucking(sample) ? DUCKING_HEIGHT : STANDING_HEIGHT;
}

export function eyeHeight(sample: Pick<TacticalSample, 'flags'>): number {
  return isDucking(sample) ? DUCKING_EYE : STANDING_EYE;
}

/** Room over the highest player before the cut, enough for a doorway and a box on top. */
const CUT_HEADROOM = 190;

/**
 * The opening height cut: just above where the round's players stood, so roofs
 * go and walls stay. With no positions, no cut.
 */
export function defaultCut(mesh: { minZ: number; maxZ: number }, frames: readonly TacticalFrame[]): number {
  let top = Number.NEGATIVE_INFINITY;
  for (const frame of frames) {
    for (const sample of frame.samples) if (sample.z > top) top = sample.z;
  }
  if (!Number.isFinite(top)) return mesh.maxZ;
  return Math.min(mesh.maxZ, Math.max(mesh.minZ, top + STANDING_HEIGHT + CUT_HEADROOM));
}

/**
 * What the cameras frame: the play area the analysis observed, at the mesh's
 * heights. Collision meshes can reach far past the playable map, so the mesh
 * bounds are used only when the analysis has none.
 */
export function playBox(mesh: WorldBox, play: RadarBounds | undefined, frames: readonly TacticalFrame[] = []): WorldBox {
  let low = Number.POSITIVE_INFINITY;
  let high = Number.NEGATIVE_INFINITY;
  for (const frame of frames) {
    for (const sample of frame.samples) {
      if (sample.z < low) low = sample.z;
      if (sample.z > high) high = sample.z;
    }
  }
  const minZ = Number.isFinite(low) ? Math.max(mesh.min.z, low - 64) : mesh.min.z;
  const maxZ = Number.isFinite(high) ? Math.min(mesh.max.z, high + STANDING_HEIGHT * 3) : mesh.max.z;
  if (play === undefined || play.max_x <= play.min_x || play.max_y <= play.min_y) {
    return { min: { ...mesh.min, z: minZ }, max: { ...mesh.max, z: Math.max(minZ, maxZ) } };
  }
  return {
    min: { x: play.min_x, y: play.min_y, z: minZ },
    max: { x: play.max_x, y: play.max_y, z: Math.max(minZ, maxZ) },
  };
}
