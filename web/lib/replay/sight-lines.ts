import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '../api/tactical.ts';
import type { TacticalEvent, TacticalSample, TacticalSide } from '../api/tactical.ts';
import { clearShare, segmentBlocked } from './sight-bvh.ts';
import type { SightBvh } from './sight-bvh.ts';
import type { Vec3 } from './three/orbit.ts';
import { SMOKE_FADE_SECONDS, SMOKE_SECONDS } from './three/utility-visuals.ts';

/** Eye height over the feet, standing and crouched (CS2 view offsets). */
const EYE_STANDING = 64;
const EYE_DUCKING = 46;
/** A body is seen when its head or its chest is. */
const CHEST_BELOW_EYE = 24;
/**
 * Horizontal and vertical field of view in degrees: CS2's 90 at 4:3 widened to
 * 16:9, and a sniper's first zoom (40) the same way.
 */
const FOV = { open: { horizontal: 106.3, vertical: 73.7 }, scoped: { horizontal: 51.7, vertical: 30.4 } } as const;
/** A smoke blocks sight inside this radius around a point this high over where it landed. */
const SMOKE_BLOCK_RADIUS = 132;
const SMOKE_CENTRE_HEIGHT = 56;
/** A smoke blocks once it has bloomed and until it starts to thin. */
const SMOKE_BLOCKS_FROM = 0.75;

export type SightPlayer = { slot: number; side: TacticalSide; eye: Vec3; chest: Vec3; yaw: number; pitch: number; scoped: boolean };

/** A pair of enemies with nothing between them; `aSees` when `b` is inside `a`'s field of view. */
export type SightPair = { a: number; b: number; aSees: boolean; bSees: boolean };

export function sightPlayer(sample: TacticalSample): SightPlayer {
  const eye = sample.z + (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.ducking) ? EYE_DUCKING : EYE_STANDING);
  return {
    slot: sample.slot,
    side: hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT) ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct,
    eye: { x: sample.x, y: sample.y, z: eye },
    chest: { x: sample.x, y: sample.y, z: eye - CHEST_BELOW_EYE },
    yaw: sample.yaw,
    pitch: sample.pitch ?? 0,
    scoped: hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.scoped),
  };
}

/** The angle from `from` to `to`, folded into -180..180. */
function angleBetween(from: number, to: number): number {
  return ((((to - from) % 360) + 540) % 360) - 180;
}

/** Whether `target` falls inside the viewer's field of view, by yaw and pitch. */
export function inFieldOfView(viewer: SightPlayer, target: Vec3): boolean {
  const dx = target.x - viewer.eye.x;
  const dy = target.y - viewer.eye.y;
  const dz = target.z - viewer.eye.z;
  const fov = viewer.scoped ? FOV.scoped : FOV.open;
  const yaw = (Math.atan2(dy, dx) * 180) / Math.PI;
  if (Math.abs(angleBetween(viewer.yaw, yaw)) > fov.horizontal / 2) return false;
  // CS2's pitch is positive looking down; elevation is positive looking up.
  const elevation = (Math.atan2(dz, Math.hypot(dx, dy)) * 180) / Math.PI;
  return Math.abs(elevation + viewer.pitch) <= fov.vertical / 2;
}

export type SightSmoke = { x: number; y: number; z: number };

/** True when the segment passes through a smoke's sphere. */
function smokeBlocks(from: Vec3, to: Vec3, smokes: readonly SightSmoke[]): boolean {
  const dx = to.x - from.x;
  const dy = to.y - from.y;
  const dz = to.z - from.z;
  const length2 = dx * dx + dy * dy + dz * dz;
  for (const smoke of smokes) {
    const cz = smoke.z + SMOKE_CENTRE_HEIGHT;
    const t = length2 > 0 ? Math.min(1, Math.max(0, ((smoke.x - from.x) * dx + (smoke.y - from.y) * dy + (cz - from.z) * dz) / length2)) : 0;
    const distance = Math.hypot(from.x + dx * t - smoke.x, from.y + dy * t - smoke.y, from.z + dz * t - cz);
    if (distance < SMOKE_BLOCK_RADIUS) return true;
  }
  return false;
}

function clear(options: { bvh: SightBvh; from: Vec3; to: Vec3; smokes: readonly SightSmoke[] }): boolean {
  return !smokeBlocks(options.from, options.to, options.smokes) && !segmentBlocked(options.bvh, options.from, options.to);
}

/**
 * Every pair of living enemies that could see each other: eye to head or eye
 * to chest, through no wall and no smoke. Two to four rays a pair.
 */
export function sightPairs(options: { bvh: SightBvh; players: readonly SightPlayer[]; smokes: readonly SightSmoke[] }): SightPair[] {
  const { bvh, players, smokes } = options;
  const pairs: SightPair[] = [];
  for (let i = 0; i < players.length; i += 1) {
    for (let j = i + 1; j < players.length; j += 1) {
      const a = players[i];
      const b = players[j];
      if (a.side === b.side) continue;
      const exposed =
        clear({ bvh, from: a.eye, to: b.eye, smokes }) ||
        clear({ bvh, from: a.eye, to: b.chest, smokes }) ||
        clear({ bvh, from: b.eye, to: a.chest, smokes });
      if (!exposed) continue;
      pairs.push({ a: a.slot, b: b.slot, aSees: inFieldOfView(a, b.chest), bSees: inFieldOfView(b, a.chest) });
    }
  }
  return pairs;
}

/**
 * The floor a player's view covers at eye height: `rays + 1` world points
 * across the horizontal field of view, each cut short at the first wall.
 */
export function viewWedge(options: { bvh: SightBvh; player: SightPlayer; reach: number; rays: number }): Vec3[] {
  const { bvh, player, reach } = options;
  const rays = Math.max(2, Math.floor(options.rays));
  const half = (player.scoped ? FOV.scoped : FOV.open).horizontal / 2;
  const points: Vec3[] = [];
  for (let i = 0; i <= rays; i += 1) {
    const angle = ((player.yaw - half + (2 * half * i) / rays) * Math.PI) / 180;
    const end = { x: player.eye.x + Math.cos(angle) * reach, y: player.eye.y + Math.sin(angle) * reach, z: player.eye.z };
    const share = clearShare(bvh, player.eye, end);
    points.push({ x: player.eye.x + (end.x - player.eye.x) * share, y: player.eye.y + (end.y - player.eye.y) * share, z: player.eye.z });
  }
  return points;
}

/** Smokes thick enough to stop sight at `nowSeconds`, from every smoke of the round. */
export function blockingSmokes(options: { events: readonly TacticalEvent[]; nowSeconds: number; secondsOf: (tick: number) => number }): SightSmoke[] {
  const out: SightSmoke[] = [];
  for (const event of options.events) {
    if (event.kind !== TACTICAL_EVENT_KINDS.smoke) continue;
    const age = options.nowSeconds - options.secondsOf(event.tick);
    const lasts = event.end_tick === undefined ? SMOKE_SECONDS : options.secondsOf(event.end_tick) - options.secondsOf(event.tick);
    if (age >= SMOKE_BLOCKS_FROM && age <= lasts - SMOKE_FADE_SECONDS) out.push({ x: event.pos[0], y: event.pos[1], z: event.pos[2] });
  }
  return out;
}

export type FrameSight = { pairs: SightPair[]; wedges: ReadonlyMap<number, Vec3[]> };

/** Sight pairs and cut view wedges for the living players of one frame. */
export function frameSight(options: { bvh: SightBvh; samples: readonly TacticalSample[]; smokes: readonly SightSmoke[]; reach: number; rays: number }): FrameSight {
  const players = options.samples.filter((sample) => hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)).map(sightPlayer);
  const wedges = new Map<number, Vec3[]>();
  for (const player of players) wedges.set(player.slot, viewWedge({ bvh: options.bvh, player, reach: options.reach, rays: options.rays }));
  return { pairs: sightPairs({ bvh: options.bvh, players, smokes: options.smokes }), wedges };
}
