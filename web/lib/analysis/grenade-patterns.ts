import type { TeamGrenade, TeamGrenadeKind } from '../api/grenades.ts';
import type { RadarCalibration } from '../api/tactical.ts';
import { onShownFloor } from './floors.ts';
import type { ShownFloor } from './floors.ts';
import { renderedToWorld } from '../tactical-spatial.ts';
import { isCalibrationUsable, worldToRendered, type RadarPoint } from '../tactical-transform.ts';
import type { ReplayRef } from './replay-link.ts';

/**
 * Pure logic of the Pattern Finder: the box a drag draws on the radar, which
 * grenades fall in it, how repeated throws group into spots, the throw time
 * histogram and the replay playlist of a spot.
 */

export const GRENADE_KIND_LABEL: Record<TeamGrenadeKind, string> = {
  smoke: 'Humo',
  flash: 'Flash',
  he: 'HE',
  molotov: 'Molotov',
  decoy: 'Señuelo',
};

/** Which end of the throw a box selects on. */
export type GrenadeArea = 'landing' | 'throw';

export type WorldBox = { minX: number; minY: number; maxX: number; maxY: number };

/** A stable id for one throw. */
export function grenadeId(grenade: TeamGrenade): string {
  return `${grenade.match_id}:${grenade.round}:${grenade.tick}:${grenade.thrower_steamid64}`;
}

/** The world box between two corners of a drag in rendered radar pixels. */
export function dragWorldBox(calibration: RadarCalibration, size: number, a: RadarPoint, b: RadarPoint): WorldBox {
  const one = renderedToWorld(calibration, a.x, a.y, size);
  const two = renderedToWorld(calibration, b.x, b.y, size);
  return {
    minX: Math.min(one.x, two.x),
    minY: Math.min(one.y, two.y),
    maxX: Math.max(one.x, two.x),
    maxY: Math.max(one.y, two.y),
  };
}

/**
 * True when the chosen end of the throw is inside the box; a throw without origin
 * has no throw end. With `floor`, that end must also be on the floor the box was drawn on.
 */
export function grenadeInBox(options: { grenade: TeamGrenade; box: WorldBox; area: GrenadeArea; floor?: ShownFloor }): boolean {
  const { grenade, box, area, floor } = options;
  const point = area === 'landing' ? grenade.landing : grenade.origin;
  if (point === null) return false;
  const [x, y, z] = point;
  if (floor !== undefined && !onShownFloor(floor, z)) return false;
  return x >= box.minX && x <= box.maxX && y >= box.minY && y <= box.maxY;
}

/** Where a throw sits on a `size` radar: from the thrower (null without origin) to the landing. */
export type GrenadeTracer = { from: RadarPoint | null; to: RadarPoint };

export function grenadeTracer(calibration: RadarCalibration, size: number, grenade: TeamGrenade): GrenadeTracer | null {
  if (!isCalibrationUsable(calibration) || !(size > 0)) return null;
  const to = worldToRendered(calibration, grenade.landing[0], grenade.landing[1], size);
  const from = grenade.origin === null ? null : worldToRendered(calibration, grenade.origin[0], grenade.origin[1], size);
  return { from, to };
}

/** Distance from a point to a segment, in the same units. */
export function segmentDistance(point: RadarPoint, a: RadarPoint, b: RadarPoint): number {
  const dx = b.x - a.x;
  const dy = b.y - a.y;
  const length = dx * dx + dy * dy;
  const t = length === 0 ? 0 : Math.max(0, Math.min(1, ((point.x - a.x) * dx + (point.y - a.y) * dy) / length));
  return Math.hypot(point.x - (a.x + t * dx), point.y - (a.y + t * dy));
}

/** Index of the tracer nearest the point within `reach` pixels, or -1. */
export function nearestTracer(tracers: readonly GrenadeTracer[], point: RadarPoint, reach: number): number {
  let best = -1;
  let bestDistance = reach;
  tracers.forEach((tracer, index) => {
    const distance = segmentDistance(point, tracer.from ?? tracer.to, tracer.to);
    if (distance <= bestDistance) {
      best = index;
      bestDistance = distance;
    }
  });
  return best;
}

/** Throws that repeat: same kind, origins and landings close together. */
export type GrenadeSpot = {
  key: string;
  kind: TeamGrenadeKind;
  grenades: TeamGrenade[];
  /** Mean throw position; null when no throw of the spot has an origin. */
  origin: [number, number] | null;
  landing: [number, number];
  throwers: { steamid64: string; name: string; count: number }[];
  /** Distinct rounds the spot was thrown in, and how many of them were won. */
  rounds: number;
  roundsWon: number;
};

/** World units: a lineup is repeated within a step or two; landings spread wider. */
export const SPOT_ORIGIN_RADIUS = 100;
export const SPOT_LANDING_RADIUS = 160;
/** Throws further apart in height than this stand or land on different floors. */
export const SPOT_HEIGHT = 200;

type Building = { kind: TeamGrenadeKind; grenades: TeamGrenade[]; origin: [number, number] | null; landing: [number, number] };

function mean(points: readonly (readonly number[])[]): [number, number] {
  let x = 0;
  let y = 0;
  for (const point of points) {
    x += point[0] ?? 0;
    y += point[1] ?? 0;
  }
  return [x / points.length, y / points.length];
}

function near(a: readonly number[], b: readonly number[], radius: number): boolean {
  return Math.hypot((a[0] ?? 0) - (b[0] ?? 0), (a[1] ?? 0) - (b[1] ?? 0)) <= radius;
}

function fits(group: Building, grenade: TeamGrenade): boolean {
  if (group.kind !== grenade.kind || !near(group.landing, grenade.landing, SPOT_LANDING_RADIUS)) return false;
  // Heights against the spot's first throw, so A above B on Nuke never merge.
  const seed = group.grenades[0];
  if (seed !== undefined && Math.abs(seed.landing[2] - grenade.landing[2]) > SPOT_HEIGHT) return false;
  // A throw without origin joins on landing alone, so old demos still group.
  if (grenade.origin === null || group.origin === null) return true;
  const thrownFrom = group.grenades.find((entry) => entry.origin !== null)?.origin;
  if (thrownFrom !== undefined && thrownFrom !== null && Math.abs(thrownFrom[2] - grenade.origin[2]) > SPOT_HEIGHT) return false;
  return near(group.origin, grenade.origin, SPOT_ORIGIN_RADIUS);
}

/**
 * Groups throws into spots, greedily in the given order, comparing each throw
 * with the running centre of every spot. Spots come back most thrown first.
 */
export function clusterGrenades(grenades: readonly TeamGrenade[]): GrenadeSpot[] {
  const groups: Building[] = [];
  for (const grenade of grenades) {
    const group = groups.find((candidate) => fits(candidate, grenade));
    if (group === undefined) {
      groups.push({
        kind: grenade.kind,
        grenades: [grenade],
        origin: grenade.origin === null ? null : [grenade.origin[0], grenade.origin[1]],
        landing: [grenade.landing[0], grenade.landing[1]],
      });
      continue;
    }
    group.grenades.push(grenade);
    const origins = group.grenades.flatMap((entry) => (entry.origin === null ? [] : [entry.origin]));
    group.origin = origins.length === 0 ? null : mean(origins);
    group.landing = mean(group.grenades.map((entry) => entry.landing));
  }
  return groups.map(toSpot).sort((a, b) => b.grenades.length - a.grenades.length || a.key.localeCompare(b.key));
}

function toSpot(group: Building): GrenadeSpot {
  const first = group.grenades[0];
  const throwers = new Map<string, { steamid64: string; name: string; count: number }>();
  const rounds = new Map<string, boolean>();
  for (const grenade of group.grenades) {
    const known = throwers.get(grenade.thrower_steamid64);
    if (known === undefined) throwers.set(grenade.thrower_steamid64, { steamid64: grenade.thrower_steamid64, name: grenade.thrower_name, count: 1 });
    else known.count += 1;
    rounds.set(`${grenade.match_id}:${grenade.round}`, grenade.round_won);
  }
  return {
    key: `${group.kind}:${first === undefined ? '' : grenadeId(first)}`,
    kind: group.kind,
    grenades: group.grenades,
    origin: group.origin,
    landing: group.landing,
    throwers: [...throwers.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name)),
    rounds: rounds.size,
    roundsWon: [...rounds.values()].filter(Boolean).length,
  };
}

/** One bar of the throw time histogram: `[start, end)` seconds after freeze end. */
export type TimeBin = { start: number; end: number; count: number };

/**
 * Throw times in bins of `width` seconds from 0 to `last`; later throws, such
 * as post-plant utility, fall in the last bin.
 */
export function throwTimeBins(grenades: readonly TeamGrenade[], width = 5, last = 115): TimeBin[] {
  const count = Math.max(1, Math.ceil(last / width));
  const bins: TimeBin[] = Array.from({ length: count }, (_, index) => ({ start: index * width, end: (index + 1) * width, count: 0 }));
  for (const grenade of grenades) {
    if (!Number.isFinite(grenade.seconds)) continue;
    const index = Math.min(count - 1, Math.max(0, Math.floor(grenade.seconds / width)));
    const bin = bins[index];
    if (bin !== undefined) bin.count += 1;
  }
  return bins;
}

/** CS2 demos tick at 64 a second; a replay opens this long before the throw. */
const LEAD_TICKS = 3 * 64;

/** One replay entry per round, starting just before the first throw of that round. */
export function spotReplayRefs(grenades: readonly TeamGrenade[]): ReplayRef[] {
  const refs = new Map<string, ReplayRef>();
  for (const grenade of grenades) {
    const key = `${grenade.match_id}:${grenade.round}`;
    const tick = Math.max(0, (grenade.throw_tick ?? grenade.tick) - LEAD_TICKS);
    const known = refs.get(key);
    if (known === undefined || (known.tick ?? 0) > tick) refs.set(key, { matchId: grenade.match_id, round: grenade.round, tick });
  }
  return [...refs.values()];
}

/** The name of the callout nearest a world point, if any is within `reach`. */
export function nearestCallout(
  callouts: readonly { name: string; center: [number, number] }[],
  point: readonly number[],
  reach = 600,
): string | null {
  let best: string | null = null;
  let bestDistance = reach;
  for (const callout of callouts) {
    const distance = Math.hypot(callout.center[0] - (point[0] ?? 0), callout.center[1] - (point[1] ?? 0));
    if (distance <= bestDistance) {
      best = callout.name;
      bestDistance = distance;
    }
  }
  return best;
}
