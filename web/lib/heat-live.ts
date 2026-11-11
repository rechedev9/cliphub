import { TACTICAL_EVENT_KINDS } from './api/tactical.ts';
import type { RadarCalibration, RadarLevel, TacticalSide } from './api/tactical.ts';
import type { OverlayRound, OverlayTrack } from './api/spatial.ts';
import { pathLevels } from './replay/floor-assign.ts';
import { isWholeRound, tramoEnd } from './replay/tramo.ts';
import type { Tramo } from './replay/tramo.ts';
import { trackPointAt } from './tactical-spatial.ts';

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
export function tramoSpan(round: OverlayRound, tramo: Tramo): { from: number; to: number } | null {
  let from = tramo.from;
  if (tramo.fromPlant === true) {
    const plant = round.events.find((event) => event.kind === TACTICAL_EVENT_KINDS.plant);
    if (plant === undefined) return null;
    from = plant.seconds;
  }
  const end = tramo.fromPlant === true || isWholeRound(tramo) || tramo.to <= 0 ? round.duration_seconds : Math.min(tramoEnd(tramo), round.duration_seconds);
  return end > from ? { from, to: end } : null;
}

// Each track's floors are settled once: a drag recomputes the heat many times over the same tracks.
const settledFloors = new WeakMap<OverlayTrack, RadarLevel[]>();

/** The floor of each point of a track, by the replay's rule (dead band, standing still drops it). */
export function trackLevels(options: { track: OverlayTrack; step: number; tickrate: number; calibration: RadarCalibration }): RadarLevel[] {
  const { track, step, tickrate, calibration } = options;
  const known = settledFloors.get(track);
  if (known !== undefined) return known;
  const ticks = track.points.map((_, i) => (track.start + i) * step * tickrate);
  const levels = pathLevels({ calibration, ticks, heights: track.points.map((point) => point[2]) });
  settledFloors.set(track, levels);
  return levels;
}

type Acc = { count: number; seconds: number[] };
type Add = (point: { x: number; y: number; level: RadarLevel }, seconds: number) => void;
type TrackRead = { track: OverlayTrack; levels: readonly RadarLevel[]; window: { from: number; to: number }; step: number; add: Add };

/** The heatmap of the tramo, or null when the calibration cannot place a mark. */
export function liveHeat(input: LiveHeatInput): LiveHeat | null {
  const { rounds, step, side, kind, tramo, calibration, grid, cellSize } = input;
  const span = calibration.size * calibration.scale;
  if (!(span > 0) || !(grid > 0) || !(step > 0)) return null;
  const floors = new Map<RadarLevel, Map<number, Acc>>();
  const add: Add = ({ x, y, level }, seconds) => {
    const column = Math.floor(((x - calibration.pos_x) / span) * grid);
    const row = Math.floor(((calibration.pos_y - y) / span) * grid);
    if (column < 0 || row < 0 || column >= grid || row >= grid) return;
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
    const window = tramoSpan(round, tramo);
    if (window === null) continue;
    for (const track of round.tracks) {
      if (track.side !== side) continue;
      const read = { track, levels: trackLevels({ track, step, tickrate: round.tickrate, calibration }), window, step, add };
      if (kind === 'position') addInstants(read);
      else addArrivals({ ...read, cellSize, arrivals });
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

/** One mark per `LIVE_SAMPLE_SECONDS` of the window while the player is alive, on the floor of the point before it. */
function addInstants({ track, levels, window, step, add }: TrackRead): void {
  const n = Math.floor((window.to - window.from) / LIVE_SAMPLE_SECONDS);
  const lead = (window.to - window.from - n * LIVE_SAMPLE_SECONDS) / 2;
  for (let i = 0; i < n; i++) {
    const seconds = window.from + lead + (i + 0.5) * LIVE_SAMPLE_SECONDS;
    const point = trackPointAt(track, seconds, step);
    if (point !== undefined) add({ x: point[0], y: point[1], level: levels[point[3]] }, seconds);
  }
}

/** The first point of the window in each cell the player reaches, with its time. */
function addArrivals(options: TrackRead & { cellSize: number; arrivals: Map<string, number[]> }): void {
  const { track, levels, window, step, add, cellSize, arrivals } = options;
  const seen = new Set<string>();
  const first = Math.max(0, Math.ceil(window.from / step - track.start));
  for (let i = first; i < track.points.length; i++) {
    const seconds = (track.start + i) * step;
    if (seconds > window.to) break;
    const [x, y] = track.points[i];
    const level = levels[i];
    const cell = `${level}|${Math.floor(x / cellSize)}|${Math.floor(y / cellSize)}`;
    if (seen.has(cell)) continue;
    seen.add(cell);
    add({ x, y, level }, seconds);
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
