import { TACTICAL_EVENT_KINDS } from './api/tactical.ts';
import type { RadarCalibration, RadarLevel, TacticalSide } from './api/tactical.ts';
import type { OverlayRound, OverlayTrack } from './api/spatial.ts';
import { levelForAltitude } from './tactical-transform.ts';
import { isWholeRound, tramoEnd } from './replay/tramo.ts';
import type { Tramo } from './replay/tramo.ts';

// The position and timing heatmaps rebuilt in the browser from the overlay's tracks, so a tramo
// drag repaints at once. They follow `tacticalplan` (heat_density.go addInstants, addArrivals).

/** One instant of the position kind stands for this long, as `HeatmapSampleSeconds`. */
export const LIVE_SAMPLE_SECONDS = 0.25;

export type LiveHeatKind = 'position' | 'timing';

/** One floor's marks on the density grid: [column, row, count], and for timing each bin's median seconds. */
export type LiveHeatLayer = { level: RadarLevel; bins: [number, number, number][]; seconds: number[] | null };

/** `maxSeconds` is the latest median first arrival of a 64-unit cell, the legend's scale for timing. */
export type LiveHeat = { layers: LiveHeatLayer[]; maxSeconds: number };

export type LiveHeatInput = {
  rounds: readonly OverlayRound[];
  step: number;
  side: TacticalSide;
  kind: LiveHeatKind;
  tramo: Tramo;
  calibration: RadarCalibration;
  grid: number;
  cellSize: number;
};

/** The part of a round the tramo keeps, in seconds after freeze end; null when none. */
export function liveSpan(round: OverlayRound, tramo: Tramo): { from: number; to: number } | null {
  let from = tramo.from;
  if (tramo.fromPlant === true) {
    const plant = round.events.find((event) => event.kind === TACTICAL_EVENT_KINDS.plant);
    if (plant === undefined) return null;
    from = plant.seconds;
  }
  const end = tramo.fromPlant === true || isWholeRound(tramo) || tramo.to <= 0 ? round.duration_seconds : Math.min(tramoEnd(tramo), round.duration_seconds);
  return end > from ? { from, to: end } : null;
}

/** Where a track is at `seconds` with its height, interpolated; undefined before it starts and after its last point. */
export function trackPointAt(track: OverlayTrack, seconds: number, step: number): [number, number, number] | undefined {
  if (!(step > 0) || track.points.length === 0) return undefined;
  const at = seconds / step - track.start;
  if (at < 0 || at > track.points.length - 1) return undefined;
  const i = Math.floor(at);
  const a = track.points[i];
  const b = track.points[Math.min(i + 1, track.points.length - 1)];
  const k = at - i;
  return [a[0] + (b[0] - a[0]) * k, a[1] + (b[1] - a[1]) * k, a[2] + (b[2] - a[2]) * k];
}

type Acc = { count: number; seconds: number[] };

/** The heatmap of the tramo, or null when the calibration cannot place a mark. */
export function liveHeat(input: LiveHeatInput): LiveHeat | null {
  const { rounds, step, side, kind, tramo, calibration, grid, cellSize } = input;
  const span = calibration.size * calibration.scale;
  if (!(span > 0) || !(grid > 0) || !(step > 0)) return null;
  const floors = new Map<RadarLevel, Map<number, Acc>>();
  const add = (point: readonly [number, number, number], seconds: number): void => {
    const column = Math.floor(((point[0] - calibration.pos_x) / span) * grid);
    const row = Math.floor(((calibration.pos_y - point[1]) / span) * grid);
    if (column < 0 || row < 0 || column >= grid || row >= grid) return;
    const level = levelForAltitude(calibration, point[2]);
    let bins = floors.get(level);
    if (bins === undefined) {
      bins = new Map();
      floors.set(level, bins);
    }
    const key = row * grid + column;
    const acc = bins.get(key);
    if (acc === undefined) bins.set(key, { count: 1, seconds: kind === 'timing' ? [seconds] : [] });
    else {
      acc.count++;
      if (kind === 'timing') acc.seconds.push(seconds);
    }
  };
  const arrivals = new Map<string, number[]>();
  for (const round of rounds) {
    const window = liveSpan(round, tramo);
    if (window === null) continue;
    for (const track of round.tracks) {
      if (track.side !== side) continue;
      if (kind === 'position') addInstants({ track, window, step, add });
      else addArrivals({ track, window, step, cellSize, calibration, add, arrivals });
    }
  }
  let maxSeconds = 0;
  for (const seconds of arrivals.values()) maxSeconds = Math.max(maxSeconds, median(seconds));
  const layers: LiveHeatLayer[] = [];
  for (const [level, bins] of floors) {
    const keys = [...bins.keys()].sort((a, b) => a - b);
    layers.push({
      level,
      bins: keys.map((key) => [key % grid, Math.floor(key / grid), bins.get(key)?.count ?? 0]),
      seconds: kind === 'timing' ? keys.map((key) => median(bins.get(key)?.seconds ?? [])) : null,
    });
  }
  return { layers, maxSeconds };
}

type Add = (point: readonly [number, number, number], seconds: number) => void;

/** One mark per `LIVE_SAMPLE_SECONDS` of the window while the player is alive. */
function addInstants({ track, window, step, add }: { track: OverlayTrack; window: { from: number; to: number }; step: number; add: Add }): void {
  const n = Math.floor((window.to - window.from) / LIVE_SAMPLE_SECONDS);
  const lead = (window.to - window.from - n * LIVE_SAMPLE_SECONDS) / 2;
  for (let i = 0; i < n; i++) {
    const seconds = window.from + lead + (i + 0.5) * LIVE_SAMPLE_SECONDS;
    const point = trackPointAt(track, seconds, step);
    if (point !== undefined) add(point, seconds);
  }
}

/** The first point of the window in each cell the player reaches, with its time. */
function addArrivals(options: {
  track: OverlayTrack;
  window: { from: number; to: number };
  step: number;
  cellSize: number;
  calibration: RadarCalibration;
  add: Add;
  arrivals: Map<string, number[]>;
}): void {
  const { track, window, step, cellSize, calibration, add, arrivals } = options;
  const seen = new Set<string>();
  const first = Math.max(0, Math.ceil(window.from / step - track.start));
  for (let i = first; i < track.points.length; i++) {
    const seconds = (track.start + i) * step;
    if (seconds > window.to) break;
    const point = track.points[i];
    const cell = `${levelForAltitude(calibration, point[2])}|${Math.floor(point[0] / cellSize)}|${Math.floor(point[1] / cellSize)}`;
    if (seen.has(cell)) continue;
    seen.add(cell);
    add(point, seconds);
    const list = arrivals.get(cell);
    if (list === undefined) arrivals.set(cell, [seconds]);
    else list.push(seconds);
  }
}

function median(values: readonly number[]): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const middle = Math.floor(sorted.length / 2);
  const value = sorted.length % 2 === 1 ? sorted[middle] : (sorted[middle - 1] + sorted[middle]) / 2;
  return Math.round(value * 10) / 10;
}
