import { TACTICAL_EVENT_KINDS } from '../../api/tactical.ts';
import type { TacticalEventKind } from '../../api/tactical.ts';
import type { TimelineEvent } from '../../tactical-timeline.ts';
import type { Vec3 } from './orbit.ts';

/** CS2 demos record 22.06 s from SmokeStart to SmokeExpired for every smoke; it thins over the last 2. */
export const SMOKE_SECONDS = 22;
export const SMOKE_FADE_SECONDS = 2;

/** How long each grenade's effect stays on the map, in seconds (CS2 defaults). */
export const UTILITY_SECONDS: Partial<Record<TacticalEventKind, number>> = {
  [TACTICAL_EVENT_KINDS.smoke]: SMOKE_SECONDS,
  [TACTICAL_EVENT_KINDS.molotov]: 7,
  [TACTICAL_EVENT_KINDS.flash]: 0.6,
  [TACTICAL_EVENT_KINDS.he]: 0.6,
  [TACTICAL_EVENT_KINDS.decoy]: 0.6,
};

/** The thrown path stays this long after the grenade lands, fading out. */
export const ARC_LINGER_SECONDS = 2;

const SMOKE_BLOOM_SECONDS = 1.2;
const FIRE_SPREAD_SECONDS = 0.5;
const FIRE_FADE_SECONDS = 1;

export type UtilityEffect = { strength: number; grow: number };

export type UtilityArc = { from: Vec3; to: Vec3; progress: number; fade: number };

/** One grenade on screen at the playhead; `index` is its place in the round's events, a stable key. */
export type UtilityVisual = {
  index: number;
  kind: TacticalEventKind;
  at: Vec3;
  /** Where it was thrown from, when the analysis recorded it. */
  from: Vec3 | null;
  effect: UtilityEffect | null;
  arc: UtilityArc | null;
};

function vec(p: readonly [number, number, number]): Vec3 {
  return { x: p[0], y: p[1], z: p[2] };
}

function clamp01(value: number): number {
  return Math.min(1, Math.max(0, value));
}

/** Strength and size of a detonated grenade `age` seconds after it went off. */
export function utilityEffect(kind: TacticalEventKind, age: number): UtilityEffect | null {
  const duration = UTILITY_SECONDS[kind];
  if (duration === undefined || age < 0 || age > duration) return null;
  if (kind === TACTICAL_EVENT_KINDS.smoke) {
    return { grow: 0.35 + 0.65 * clamp01(age / SMOKE_BLOOM_SECONDS), strength: clamp01((duration - age) / SMOKE_FADE_SECONDS) };
  }
  if (kind === TACTICAL_EVENT_KINDS.molotov) {
    return { grow: 0.4 + 0.6 * clamp01(age / FIRE_SPREAD_SECONDS), strength: clamp01((duration - age) / FIRE_FADE_SECONDS) };
  }
  const t = age / duration;
  return { grow: 0.3 + 0.7 * t, strength: 1 - t };
}

/**
 * Grenades worth drawing at `nowSeconds`: in flight, landed with the path still
 * fading, or with their effect running. `secondsOf` turns a tick into bar seconds.
 */
export function utilityVisuals(
  events: readonly TimelineEvent[],
  nowSeconds: number,
  secondsOf: (tick: number) => number,
): UtilityVisual[] {
  const out: UtilityVisual[] = [];
  events.forEach((entry, index) => {
    const { event } = entry;
    const duration = UTILITY_SECONDS[event.kind];
    if (duration === undefined) return;
    const at = vec(event.pos);
    let arc: UtilityArc | null = null;
    if (event.throw !== undefined) {
      const thrown = secondsOf(event.throw.tick);
      const flight = entry.seconds - thrown;
      if (flight > 0 && nowSeconds >= thrown && nowSeconds <= entry.seconds + ARC_LINGER_SECONDS) {
        arc = {
          from: vec(event.throw.pos),
          to: at,
          progress: clamp01((nowSeconds - thrown) / flight),
          fade: nowSeconds <= entry.seconds ? 1 : 1 - (nowSeconds - entry.seconds) / ARC_LINGER_SECONDS,
        };
      }
    }
    const effect = utilityEffect(event.kind, nowSeconds - entry.seconds);
    if (arc === null && effect === null) return;
    out.push({ index, kind: event.kind, at, from: event.throw === undefined ? null : vec(event.throw.pos), effect, arc });
  });
  return out;
}

/**
 * A throw's path as `steps + 1` points: a parabola-like curve that peaks over
 * the higher end. An estimate of the flight, never the recorded one.
 */
export function grenadeArc(from: Vec3, to: Vec3, steps: number): Vec3[] {
  const count = Math.max(1, Math.floor(steps));
  const horizontal = Math.hypot(to.x - from.x, to.y - from.y);
  const apex = Math.max(from.z, to.z) + Math.min(420, Math.max(24, horizontal * 0.22));
  const controlZ = 2 * apex - 0.5 * (from.z + to.z);
  const points: Vec3[] = [];
  for (let i = 0; i <= count; i += 1) {
    const t = i / count;
    const a = (1 - t) * (1 - t);
    const b = 2 * (1 - t) * t;
    const c = t * t;
    points.push({
      x: from.x + (to.x - from.x) * t,
      y: from.y + (to.y - from.y) * t,
      z: a * from.z + b * controlZ + c * to.z,
    });
  }
  return points;
}

export type BombVisual =
  | { state: 'planted' | 'defused'; at: Vec3 }
  | { state: 'exploded'; at: Vec3; effect: UtilityEffect }
  | { state: 'dropped'; at: Vec3 };

const EXPLOSION_SECONDS = 1.5;

/** The bomb on the ground at the playhead; a carried bomb is drawn on its carrier instead. */
export function bombVisual(events: readonly TimelineEvent[], nowSeconds: number, carried: boolean): BombVisual | null {
  let state: BombVisual | null = null;
  for (const { event, seconds } of events) {
    if (seconds > nowSeconds) break;
    const at = vec(event.pos);
    if (event.kind === TACTICAL_EVENT_KINDS.plant) state = { state: 'planted', at };
    else if (event.kind === TACTICAL_EVENT_KINDS.defuse && state !== null) state = { state: 'defused', at: state.at };
    else if (event.kind === TACTICAL_EVENT_KINDS.bombDrop && (state === null || state.state === 'dropped')) state = { state: 'dropped', at };
    else if (event.kind === TACTICAL_EVENT_KINDS.explode) {
      const age = nowSeconds - seconds;
      const site: Vec3 = state === null ? at : state.at;
      state = age > EXPLOSION_SECONDS ? null : { state: 'exploded', at: site, effect: { grow: 0.3 + 0.7 * (age / EXPLOSION_SECONDS), strength: 1 - age / EXPLOSION_SECONDS } };
    }
  }
  if (state?.state === 'dropped' && carried) return null;
  return state;
}

/** One soft blob of a smoke or one tongue of a fire, in units of the effect's radius and height. */
export type Puff = { x: number; y: number; z: number; size: number; phase: number };

/** A small deterministic generator, so a grenade looks the same on every frame and every replay. */
export function seededRandom(seed: number): () => number {
  let state = (Math.floor(seed) ^ 0x9e3779b9) >>> 0;
  return () => {
    state = (state + 0x6d2b79f5) >>> 0;
    let t = state;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

/**
 * `count` blobs filling a dome: x and y within the unit disc, z from 0 to 1,
 * bigger and lower blobs near the middle so the cloud sits heavy on the floor.
 */
export function puffLayout(seed: number, count: number): Puff[] {
  const random = seededRandom(seed);
  const puffs: Puff[] = [];
  for (let i = 0; i < count; i += 1) {
    const angle = (i / count) * Math.PI * 2 + random() * 0.9;
    const reach = Math.sqrt(random()) * 0.78;
    const dome = Math.sqrt(Math.max(0, 1 - reach * reach));
    puffs.push({
      x: Math.cos(angle) * reach,
      y: Math.sin(angle) * reach,
      z: (0.25 + random() * 0.55) * dome,
      size: 0.75 + (1 - reach) * 0.45 + random() * 0.2,
      phase: random() * Math.PI * 2,
    });
  }
  return puffs;
}

/** How tall a flame stands at `seconds`, between 0.6 and 1.2: two out-of-step waves so it never pulses evenly. */
export function flicker(seconds: number, phase: number): number {
  return 0.9 + 0.2 * Math.sin(seconds * 9.1 + phase) + 0.1 * Math.sin(seconds * 23.7 + phase * 2.3);
}
