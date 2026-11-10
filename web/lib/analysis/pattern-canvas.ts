import type { OverlayRound, OverlayTrack } from '../api/spatial.ts';
import type { RadarCalibration } from '../api/tactical.ts';
import { ROUND_CLOCK_SECONDS, isWholeRound, roundClock, tramoEnd } from '../replay/tramo.ts';
import type { Tramo } from '../replay/tramo.ts';
import { worldToRendered } from '../tactical-transform.ts';
import type { WorldBox } from './grenade-patterns.ts';

/** The width of one bar of the Buscador's histogram, in seconds of the round clock. */
export const BUCKET_SECONDS = 10;

export type Bucket = {
  /** Seconds since freeze end where the bucket starts and ends. */
  start: number;
  end: number;
  total: number;
  byKey: Map<string, number>;
};

/**
 * Items counted by when they happen, in buckets over the round clock (1:55 to 0:00).
 * A moment past the clock's end (the bomb's last seconds) counts in the last bucket.
 */
export function clockHistogram<T>(options: { items: readonly T[]; secondsOf: (item: T) => number; keyOf: (item: T) => string }): { buckets: Bucket[]; tallest: number } {
  const { items, secondsOf, keyOf } = options;
  const count = Math.ceil(ROUND_CLOCK_SECONDS / BUCKET_SECONDS);
  const buckets: Bucket[] = Array.from({ length: count }, (_, index) => ({
    start: index * BUCKET_SECONDS,
    end: Math.min(ROUND_CLOCK_SECONDS, (index + 1) * BUCKET_SECONDS),
    total: 0,
    byKey: new Map<string, number>(),
  }));
  for (const item of items) {
    const seconds = secondsOf(item);
    if (!Number.isFinite(seconds)) continue;
    const bucket = buckets[Math.min(count - 1, Math.max(0, Math.floor(seconds / BUCKET_SECONDS)))];
    if (bucket === undefined) continue;
    const key = keyOf(item);
    bucket.total += 1;
    bucket.byKey.set(key, (bucket.byKey.get(key) ?? 0) + 1);
  }
  return { buckets, tallest: buckets.reduce((top, bucket) => Math.max(top, bucket.total), 0) };
}

/** "1:55 a 1:45", the clock span of a bucket. */
export function bucketLabel(bucket: Pick<Bucket, 'start' | 'end'>): string {
  return `${roundClock(bucket.start)} a ${roundClock(bucket.end)}`;
}

/** A player has left the spot they started the round on once they are this far from it, in world units. */
export const MOVE_DISTANCE = 64;

type TrackShape = Pick<OverlayTrack, 'start' | 'points'>;

/** Seconds since freeze end at which the track first leaves its starting spot, or null if it never does. */
export function firstMovement(track: TrackShape, step: number): number | null {
  const first = track.points[0];
  if (first === undefined || step <= 0) return null;
  for (let index = 1; index < track.points.length; index += 1) {
    const point = track.points[index];
    if (point !== undefined && Math.hypot(point[0] - first[0], point[1] - first[1]) >= MOVE_DISTANCE) return (track.start + index) * step;
  }
  return null;
}

export type MovementRow = {
  matchId: string;
  round: number;
  steamid64: string;
  /** Seconds since freeze end. */
  seconds: number;
  /** The tick to open the replay at, a few seconds before the player moves. */
  tick: number;
};

const LEAD_SECONDS = 3;

/** The first movement of every chosen player in every overlaid round, inside the window, newest rounds as the overlay lists them. */
export function firstMovements(options: { rounds: readonly OverlayRound[]; step: number; players: readonly string[]; tramo: Tramo }): MovementRow[] {
  const { rounds, step, players, tramo } = options;
  const rows: MovementRow[] = [];
  for (const round of rounds) {
    for (const track of round.tracks) {
      if (players.length > 0 && !players.includes(track.steamid64)) continue;
      const seconds = firstMovement(track, step);
      if (seconds === null || !inWindow(tramo, seconds)) continue;
      const lead = Math.round(LEAD_SECONDS * (round.tickrate > 0 ? round.tickrate : 64));
      rows.push({
        matchId: round.match_id,
        round: round.round,
        steamid64: track.steamid64,
        seconds,
        tick: Math.max(round.tick_freeze_end, round.tick_freeze_end + Math.round(seconds * round.tickrate) - lead),
      });
    }
  }
  return rows;
}

function inWindow(tramo: Tramo, seconds: number): boolean {
  if (isWholeRound(tramo) || tramo.fromPlant === true) return true;
  return seconds >= tramo.from && seconds <= tramoEnd(tramo);
}

/** Whether a track passes through the box while the window is open; every floor counts. */
export function trackEntersBox(options: { track: TrackShape; step: number; box: WorldBox; tramo: Tramo }): boolean {
  const { track, step, box, tramo } = options;
  return track.points.some((point, index) => {
    if (point[0] < box.minX || point[0] > box.maxX || point[1] < box.minY || point[1] > box.maxY) return false;
    return inWindow(tramo, (track.start + index) * step);
  });
}

/** The overlaid rounds in which any chosen player (everyone with none) enters the box within the window. */
export function roundsThroughBox(options: { rounds: readonly OverlayRound[]; step: number; players: readonly string[]; box: WorldBox; tramo: Tramo }): OverlayRound[] {
  const { rounds, step, players, box, tramo } = options;
  return rounds.filter((round) =>
    round.tracks.some((track) => (players.length === 0 || players.includes(track.steamid64)) && trackEntersBox({ track, step, box, tramo })),
  );
}

/** The bins of a density grid whose square lies inside the box; the box is in world units. */
export function binsInBox(options: {
  bins: readonly (readonly [number, number, number])[];
  grid: number;
  calibration: RadarCalibration;
  box: WorldBox;
}): [number, number, number][] {
  const { bins, grid, calibration, box } = options;
  const a = worldToRendered(calibration, box.minX, box.maxY, grid);
  const b = worldToRendered(calibration, box.maxX, box.minY, grid);
  const left = Math.min(a.x, b.x);
  const right = Math.max(a.x, b.x);
  const top = Math.min(a.y, b.y);
  const bottom = Math.max(a.y, b.y);
  return bins.filter(([column, row]) => column + 0.5 >= left && column + 0.5 <= right && row + 0.5 >= top && row + 0.5 <= bottom).map(([column, row, count]) => [column, row, count]);
}
