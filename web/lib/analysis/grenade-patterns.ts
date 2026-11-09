import type { TeamGrenade, TeamGrenadeKind } from '../api/grenades.ts';
import type { RadarCalibration } from '../api/tactical.ts';
import { onShownFloor } from './floors.ts';
import type { ShownFloor } from './floors.ts';
import { renderedToWorld } from '../tactical-spatial.ts';
import { isCalibrationUsable, worldToRendered, type RadarPoint } from '../tactical-transform.ts';
import type { ReplayRef } from './replay-link.ts';
import { ROUND_CLOCK_SECONDS } from '../replay/tramo.ts';

/**
 * Pure logic of the Pattern Finder: the box a drag draws on the radar, which
 * grenades fall in it, the spots the server grouped, the round clock
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

/** Throws that repeat, grouped by the orchestrator (`tacticalplan.assignSpots`): same kind, origins and landings close together. */
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
  /** The callout tokens most of its throws land at and are thrown from; empty when none names one. */
  landPlace: string;
  throwPlace: string;
};

function mostNamed(grenades: readonly TeamGrenade[], name: (grenade: TeamGrenade) => string): string {
  const counts = new Map<string, number>();
  let best = '';
  for (const grenade of grenades) {
    const value = name(grenade);
    if (value === '') continue;
    const count = (counts.get(value) ?? 0) + 1;
    counts.set(value, count);
    const top = counts.get(best) ?? 0;
    if (count > top || (count === top && value < best)) best = value;
  }
  return best;
}

function mean(points: readonly (readonly number[])[]): [number, number] {
  let x = 0;
  let y = 0;
  for (const point of points) {
    x += point[0] ?? 0;
    y += point[1] ?? 0;
  }
  return [x / points.length, y / points.length];
}

/** The grenades by the spot the server named, most rounds first; a grenade without a key is its own spot. */
export function groupSpots(grenades: readonly TeamGrenade[]): GrenadeSpot[] {
  const groups = new Map<string, TeamGrenade[]>();
  for (const grenade of grenades) {
    const key = grenade.spot === '' ? `${grenade.kind}:${grenadeId(grenade)}` : grenade.spot;
    const group = groups.get(key);
    if (group === undefined) groups.set(key, [grenade]);
    else group.push(grenade);
  }
  return [...groups.entries()]
    .map(([key, members]) => toSpot(key, members))
    .sort((a, b) => b.rounds - a.rounds || b.grenades.length - a.grenades.length || a.key.localeCompare(b.key));
}

function toSpot(key: string, grenades: TeamGrenade[]): GrenadeSpot {
  const throwers = new Map<string, { steamid64: string; name: string; count: number }>();
  const rounds = new Map<string, boolean>();
  for (const grenade of grenades) {
    const known = throwers.get(grenade.thrower_steamid64);
    if (known === undefined) throwers.set(grenade.thrower_steamid64, { steamid64: grenade.thrower_steamid64, name: grenade.thrower_name, count: 1 });
    else known.count += 1;
    rounds.set(`${grenade.match_id}:${grenade.round}`, grenade.round_won);
  }
  const origins = grenades.flatMap((grenade) => (grenade.origin === null ? [] : [grenade.origin]));
  return {
    key,
    kind: grenades[0]?.kind ?? 'smoke',
    grenades,
    origin: origins.length === 0 ? null : mean(origins),
    landing: mean(grenades.map((grenade) => grenade.landing)),
    throwers: [...throwers.values()].sort((a, b) => b.count - a.count || a.name.localeCompare(b.name)),
    rounds: rounds.size,
    roundsWon: [...rounds.values()].filter(Boolean).length,
    landPlace: mostNamed(grenades, (grenade) => grenade.land_place),
    throwPlace: mostNamed(grenades, (grenade) => grenade.throw_place),
  };
}

/** One bar of the round clock histogram: `[start, end)` seconds after freeze end, with the throws of each player. */
export type ClockBin = { start: number; end: number; count: number; byPlayer: ReadonlyMap<string, number> };

/**
 * Throw times in bins of `width` seconds over the round clock (1:55 to 0:00);
 * later throws, such as post-plant utility, fall in the last bin.
 */
export function clockBins(grenades: readonly TeamGrenade[], width = 5): ClockBin[] {
  const count = Math.max(1, Math.ceil(ROUND_CLOCK_SECONDS / width));
  const bins = Array.from({ length: count }, (_, index) => ({ start: index * width, end: (index + 1) * width, count: 0, byPlayer: new Map<string, number>() }));
  for (const grenade of grenades) {
    if (!Number.isFinite(grenade.seconds)) continue;
    const bin = bins[Math.min(count - 1, Math.max(0, Math.floor(grenade.seconds / width)))];
    if (bin === undefined) continue;
    bin.count += 1;
    bin.byPlayer.set(grenade.thrower_steamid64, (bin.byPlayer.get(grenade.thrower_steamid64) ?? 0) + 1);
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

/** One player who threw: how many of the grenades are theirs. */
export type GrenadeThrower = { steamid64: string; name: string; throws: number };

/** Who threw the grenades, most throws first, so a player keeps the same colour while the filters change. */
export function throwersOf(grenades: readonly TeamGrenade[]): GrenadeThrower[] {
  const out = new Map<string, GrenadeThrower>();
  for (const grenade of grenades) {
    const known = out.get(grenade.thrower_steamid64);
    if (known === undefined) out.set(grenade.thrower_steamid64, { steamid64: grenade.thrower_steamid64, name: grenade.thrower_name, throws: 1 });
    else known.throws += 1;
  }
  return [...out.values()].sort((a, b) => b.throws - a.throws || a.name.localeCompare(b.name));
}

/** A click picks one player (again: everyone); with Shift it adds or drops one. */
export function pickThrower(picked: readonly string[], steamid64: string, additive: boolean): string[] {
  if (additive) return picked.includes(steamid64) ? picked.filter((entry) => entry !== steamid64) : [...picked, steamid64];
  return picked.length === 1 && picked[0] === steamid64 ? [] : [steamid64];
}
