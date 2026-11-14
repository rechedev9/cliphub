import type { RadarLevel } from '../api/tactical.ts';
import type { TrailPoint } from '../tactical-replay.ts';

/** How many strengths a trail fades through; a stroke per strength instead of one per segment. */
export const TRAIL_STEPS = 8;

const TAIL_ALPHA = 0.05;
const RAMP_ALPHA = 0.35;

export type TrailStroke = { alpha: number; segments: (readonly [TrailPoint, TrailPoint])[] };

export type TrailStrokeOptions = {
  /** The floor the plate shows; a point without one stands on it. */
  level: RadarLevel;
  /** How much of a stretch wholly on the other floor shows. */
  offFloorAlpha: number;
};

/**
 * One player's trail as a few strokes, oldest and faintest first. Each segment's strength is the fade from
 * tail to head, snapped to one of `TRAIL_STEPS`, so a multi-round frame strokes tens of paths, not hundreds.
 */
export function trailStrokes(points: readonly TrailPoint[], { level, offFloorAlpha }: TrailStrokeOptions): TrailStroke[] {
  if (points.length < 2) return [];
  const last = points.length - 1;
  // It runs for every player of every superposed round on each frame: fixed slots, no keys, no sort.
  const onFloor: (TrailStroke | undefined)[] = new Array(TRAIL_STEPS);
  const offFloor: (TrailStroke | undefined)[] = new Array(TRAIL_STEPS);
  for (let i = 1; i <= last; i += 1) {
    const before = points[i - 1].level ?? level;
    const after = points[i].level ?? level;
    const shown = before === level || after === level;
    const strength = shown ? 1 : offFloorAlpha;
    if (strength <= 0) continue;
    const step = Math.min(TRAIL_STEPS - 1, Math.floor((i / last) * TRAIL_STEPS - 1e-9));
    const slots = shown ? onFloor : offFloor;
    let stroke = slots[step];
    if (stroke === undefined) {
      stroke = { alpha: (TAIL_ALPHA + RAMP_ALPHA * ((step + 0.5) / TRAIL_STEPS)) * strength, segments: [] };
      slots[step] = stroke;
    }
    stroke.segments.push([points[i - 1], points[i]]);
  }
  const strokes: TrailStroke[] = [];
  for (let step = 0; step < TRAIL_STEPS; step += 1) {
    const off = offFloor[step];
    const on = onFloor[step];
    if (off !== undefined) strokes.push(off);
    if (on !== undefined) strokes.push(on);
  }
  return strokes;
}
