import { TACTICAL_SAMPLE_FLAGS, hasSampleFlags } from '../../api/tactical.ts';
import type { TacticalSample } from '../../api/tactical.ts';
import type { Vec3 } from './orbit.ts';
import { STANDING_HEIGHT } from './scene-math.ts';

/**
 * Where the 3D camera looks while it follows the round: the living players'
 * centre and how far they spread, the distance that keeps them all on screen,
 * the roof cut over their heads, and the easing that moves the camera there.
 */

export type ActionFocus = { centre: Vec3; radius: number };

/** Players further than this from the median are left out, so one lurker does not zoom the whole map out. */
const OUTLIER_UNITS = 1800;
const MIN_RADIUS = 450;
/** Headroom over the highest player of the action before the roofs go: under most ceilings. */
const CUT_HEADROOM = 60;

function median(values: number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)] ?? 0;
}

type Placed = Pick<TacticalSample, 'x' | 'y' | 'z' | 'flags'>;

/** The living players round the median, without the far lurkers. */
function actionGroup<S extends Placed>(samples: readonly S[]): S[] {
  const alive = samples.filter((sample) => hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive));
  if (alive.length === 0) return [];
  const mx = median(alive.map((sample) => sample.x));
  const my = median(alive.map((sample) => sample.y));
  const near = alive.filter((sample) => Math.hypot(sample.x - mx, sample.y - my) <= OUTLIER_UNITS);
  return near.length > 0 ? near : alive;
}

/** The centre and spread of the living players, or null when nobody is alive. */
export function actionFocus(samples: readonly Placed[]): ActionFocus | null {
  const group = actionGroup(samples);
  if (group.length === 0) return null;
  const centre = {
    x: group.reduce((sum, sample) => sum + sample.x, 0) / group.length,
    y: group.reduce((sum, sample) => sum + sample.y, 0) / group.length,
    z: group.reduce((sum, sample) => sum + sample.z, 0) / group.length,
  };
  const radius = group.reduce((max, sample) => Math.max(max, Math.hypot(sample.x - centre.x, sample.y - centre.y)), 0);
  return { centre, radius: Math.max(MIN_RADIUS, radius) };
}

/** Camera distance that keeps a circle of `radius` on screen at a vertical field of view, with some air. */
export function focusDistance(radius: number, fovDegrees: number): number {
  const half = (fovDegrees * Math.PI) / 360;
  return (radius * 1.35) / Math.tan(half);
}

/** The roof cut just over the highest player of the action, kept inside the mesh. */
export function liveCut(mesh: { minZ: number; maxZ: number }, samples: readonly Placed[]): number {
  const group = actionGroup(samples);
  if (group.length === 0) return mesh.maxZ;
  const top = Math.max(...group.map((sample) => sample.z));
  return Math.min(mesh.maxZ, Math.max(mesh.minZ, top + STANDING_HEIGHT + CUT_HEADROOM));
}

/** Exponential approach: how far to move toward a target in `dt` seconds with time constant `tau`. */
export function easeFactor(dt: number, tau: number): number {
  if (!(dt > 0)) return 0;
  return 1 - Math.exp(-dt / Math.max(1e-3, tau));
}

/** Smooth in and out over 0..1, for camera transitions. */
export function easeInOut(t: number): number {
  const x = Math.min(1, Math.max(0, t));
  return x < 0.5 ? 4 * x * x * x : 1 - (-2 * x + 2) ** 3 / 2;
}
