import { RADAR_LEVELS, TACTICAL_SIDES } from './api/tactical.ts';
import type { RadarCalibration, RadarLevel, TacticalSide } from './api/tactical.ts';
import type { Annotation, Heatmap, HeatmapCell, HeatmapKind, OverlayRound, OverlayTrack } from './api/spatial.ts';
import type { RadarPoint, RadarRect } from './tactical-transform.ts';

/**
 * Pure helpers of the multi-round replay: where the radar square sits in its
 * box, the inverse transform a pointer needs, how a heatmap layer is read, and
 * where an overlaid track is at a moment of the shared clock.
 */

/** How the radar square of `size` CSS pixels sits in a box so its `view` window fills it. */
export type RadarProjection = { size: number; offsetX: number; offsetY: number };

/** The same fit the 2D replay uses: the window scaled to the box and centred. */
export function radarProjection(box: { width: number; height: number }, view: RadarRect): RadarProjection {
  const width = Math.max(1, box.width);
  const height = Math.max(1, box.height);
  const size = Math.max(1, Math.floor(Math.min(width / view.width, height / view.height)));
  return {
    size,
    offsetX: Math.round((width - size * view.width) / 2 - size * view.x),
    offsetY: Math.round((height - size * view.height) / 2 - size * view.y),
  };
}

/** The inverse of `worldToRendered`: a point of a `size` square back to world x and y. */
export function renderedToWorld(calibration: RadarCalibration, x: number, y: number, size: number): { x: number; y: number } {
  const native = calibration.size / size;
  return {
    x: x * native * calibration.scale + calibration.pos_x,
    y: calibration.pos_y - y * native * calibration.scale,
  };
}

export const HEATMAP_KIND_LABEL: Record<HeatmapKind, string> = {
  position: 'Posiciones',
  death: 'Muertes',
  utility: 'Utilidad',
  timing: 'Tiempo de llegada',
};

/** What one unit of a cell is, for the legend and the cell detail. */
export const HEATMAP_UNIT_LABEL: Record<HeatmapKind, string> = {
  position: 'muestras',
  death: 'muertes',
  utility: 'granadas',
  timing: 'llegadas',
};

export const LEVEL_LABEL: Record<RadarLevel, string> = {
  [RADAR_LEVELS.default]: 'Piso superior',
  [RADAR_LEVELS.lower]: 'Piso inferior',
};

/** One side and level of a heatmap: its cells and the scale they are read on. */
export type HeatmapLayer = { cells: HeatmapCell[]; maxCount: number; total: number; rounds: number; maxSeconds: number };

export function heatmapLayer(heatmap: Heatmap, side: TacticalSide, level: string): HeatmapLayer {
  const cells = heatmap.cells.filter((cell) => cell.side === side && cell.level === level);
  const total = heatmap.sides.find((entry) => entry.side === side && entry.level === level);
  return {
    cells,
    maxCount: total?.max_count ?? 0,
    total: total?.total ?? 0,
    rounds: total?.rounds ?? 0,
    maxSeconds: cells.reduce((max, cell) => Math.max(max, cell.seconds), 0),
  };
}

/** The levels a heatmap has cells on, upper first. */
export function heatmapLevels(heatmap: Heatmap): RadarLevel[] {
  const present = new Set(heatmap.sides.map((entry) => entry.level));
  return [RADAR_LEVELS.default, RADAR_LEVELS.lower].filter((level) => present.has(level));
}

/** The side to open on: the asked one if it has data, else the side with more. */
export function heatmapSide(heatmap: Heatmap, asked: TacticalSide | ''): TacticalSide {
  if (asked !== '') return asked;
  const totals = { [TACTICAL_SIDES.ct]: 0, [TACTICAL_SIDES.t]: 0 };
  for (const entry of heatmap.sides) totals[entry.side] += entry.total;
  return totals[TACTICAL_SIDES.t] > totals[TACTICAL_SIDES.ct] ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
}

/**
 * A cell's strength from 0 to 1. Counts are long-tailed, so the ramp is on the
 * square root; for timing the early cells are the strong ones.
 */
export function heatStrength(kind: HeatmapKind, cell: HeatmapCell, layer: HeatmapLayer): number {
  if (kind === 'timing') {
    if (layer.maxSeconds <= 0) return 1;
    return 1 - Math.min(1, Math.max(0, cell.seconds / layer.maxSeconds)) * 0.85;
  }
  if (layer.maxCount <= 0) return 0;
  return Math.sqrt(Math.min(1, cell.count / layer.maxCount));
}

/** The cell under a world point, if the layer has one there. */
export function heatmapCellAt(layer: HeatmapLayer, cellSize: number, worldX: number, worldY: number): HeatmapCell | undefined {
  if (!(cellSize > 0)) return undefined;
  const x = Math.floor(worldX / cellSize);
  const y = Math.floor(worldY / cellSize);
  return layer.cells.find((cell) => cell.x === x && cell.y === y);
}

/** Elapsed time from freeze end as `m:ss`, the overlay's clock. */
export function elapsedLabel(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '0:00';
  const whole = Math.floor(seconds);
  return `${Math.floor(whole / 60)}:${String(whole % 60).padStart(2, '0')}`;
}

/** The overlay clock's end: the longest round drawn. */
export function overlayDuration(rounds: readonly OverlayRound[]): number {
  return rounds.reduce((max, round) => Math.max(max, round.duration_seconds), 0);
}

/** Stable key of an overlaid round. */
export function overlayRoundKey(round: Pick<OverlayRound, 'match_id' | 'round'>): string {
  return `${round.match_id}:${round.round}`;
}

/**
 * Where a track is at `seconds` after freeze end, interpolated between its
 * points; undefined before it starts and after its last point.
 */
export function trackPositionAt(track: OverlayTrack, seconds: number, step: number): [number, number] | undefined {
  if (!(step > 0) || track.points.length === 0) return undefined;
  const at = seconds / step - track.start;
  if (at < 0 || at > track.points.length - 1) return undefined;
  const i = Math.floor(at);
  const from = track.points[i];
  const to = track.points[Math.min(i + 1, track.points.length - 1)];
  const t = at - i;
  return [from[0] + (to[0] - from[0]) * t, from[1] + (to[1] - from[1]) * t];
}

/** The track's path from its start (or `trailSeconds` back) up to `seconds`. */
export function trackTrail(track: OverlayTrack, seconds: number, step: number, trailSeconds: number): [number, number][] {
  if (!(step > 0)) return [];
  const last = Math.min(track.points.length - 1, Math.floor(seconds / step - track.start));
  const first = Math.max(0, Math.ceil((seconds - trailSeconds) / step - track.start));
  const out: [number, number][] = [];
  for (let i = first; i <= last; i++) out.push([track.points[i][0], track.points[i][1]]);
  const head = trackPositionAt(track, seconds, step);
  if (head !== undefined && out.length > 0) out.push(head);
  return out;
}

/** Whether the track ended with a death at or before `seconds`. */
export function trackDeadAt(track: OverlayTrack, seconds: number, step: number): boolean {
  return track.died && seconds / step - track.start > track.points.length - 1;
}

/** A note's moment as seconds after its round's freeze end. */
export function noteSeconds(note: Pick<Annotation, 'tick'>, round: { tick_freeze_end: number; tickrate: number }): number {
  const rate = round.tickrate > 0 ? round.tickrate : 64;
  return (note.tick - round.tick_freeze_end) / rate;
}

/** A drawing stays on the radar this long either side of its tick. */
export const NOTE_VISIBLE_SECONDS = 3;

/** Whether a note's drawing shows at the playhead. */
export function noteVisibleAt(noteAt: number, now: number): boolean {
  return Math.abs(noteAt - now) <= NOTE_VISIBLE_SECONDS;
}

/**
 * Drops the points of a freehand stroke that sit closer than `minDistance`
 * to the last kept one, always keeping the last point, so a slow drag does not
 * store hundreds of points on one spot.
 */
export function thinStroke(points: readonly [number, number][], minDistance: number): [number, number][] {
  if (points.length <= 2) return points.map((p) => [p[0], p[1]]);
  const out: [number, number][] = [[points[0][0], points[0][1]]];
  for (let i = 1; i < points.length - 1; i++) {
    const last = out[out.length - 1];
    if (Math.hypot(points[i][0] - last[0], points[i][1] - last[1]) >= minDistance) out.push([points[i][0], points[i][1]]);
  }
  const end = points[points.length - 1];
  out.push([end[0], end[1]]);
  return out;
}

/** An SVG path through projected points. */
export function strokePath(points: readonly RadarPoint[]): string {
  return points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ');
}

/** The three corners of an arrowhead at `to`, pointing away from `from`. */
export function arrowHead(from: RadarPoint, to: RadarPoint, length: number): string {
  const angle = Math.atan2(to.y - from.y, to.x - from.x);
  const spread = Math.PI / 7;
  const left = { x: to.x - length * Math.cos(angle - spread), y: to.y - length * Math.sin(angle - spread) };
  const right = { x: to.x - length * Math.cos(angle + spread), y: to.y - length * Math.sin(angle + spread) };
  return [to, left, right].map((p) => `${p.x.toFixed(1)},${p.y.toFixed(1)}`).join(' ');
}

type CalloutGeometry = {
  cell_size: number;
  callouts: readonly {
    name: string;
    level: string;
    center: [number, number];
    cells?: [number, number][];
    stacked?: [number, number, number][];
  }[];
};

/**
 * The place owning a grid cell. In a cell shared by two floors the place whose
 * altitude there is nearest `z` wins; without `z`, the cell's busiest place.
 */
function cellOwner(geometry: CalloutGeometry, level: string, cx: number, cy: number, z: number | undefined): string {
  if (z !== undefined) {
    let best = '';
    let bestGap = Infinity;
    for (const callout of geometry.callouts) {
      if (callout.level !== level) continue;
      const entry = callout.stacked?.find(([x, y]) => x === cx && y === cy);
      if (entry === undefined) continue;
      const gap = Math.abs(z - entry[2]);
      if (gap < bestGap || (gap === bestGap && callout.name < best)) {
        best = callout.name;
        bestGap = gap;
      }
    }
    if (best !== '') return best;
  }
  for (const callout of geometry.callouts) {
    if (callout.level !== level) continue;
    if (callout.cells?.some(([x, y]) => x === cx && y === cy)) return callout.name;
  }
  return '';
}

/**
 * The place CS2 reported most in the grid cell of a world point, or in its
 * closest owned neighbour; off the grid, the callout whose centre of play is
 * closest, preferring the same level. Mirrors `tacticalplan.NearestCallout`,
 * which always has the altitude `z`.
 */
export function nearestCallout(geometry: CalloutGeometry, level: string, x: number, y: number, z?: number): string {
  const size = geometry.cell_size;
  if (size > 0) {
    const cx = Math.floor(x / size);
    const cy = Math.floor(y / size);
    const own = cellOwner(geometry, level, cx, cy, z);
    if (own !== '') return own;
    let near = '';
    let nearDistance = Infinity;
    for (let dx = -1; dx <= 1; dx++) {
      for (let dy = -1; dy <= 1; dy++) {
        const name = cellOwner(geometry, level, cx + dx, cy + dy, z);
        if (name === '') continue;
        const d = Math.hypot(x - (cx + dx + 0.5) * size, y - (cy + dy + 0.5) * size);
        if (d < nearDistance || (d === nearDistance && name < near)) {
          near = name;
          nearDistance = d;
        }
      }
    }
    if (near !== '') return near;
  }
  const { callouts } = geometry;
  for (const sameLevel of [true, false]) {
    let best = '';
    let bestDistance = Infinity;
    for (const callout of callouts) {
      if (sameLevel && callout.level !== level) continue;
      const d = (callout.center[0] - x) ** 2 + (callout.center[1] - y) ** 2;
      if (d < bestDistance || (d === bestDistance && callout.name < best)) {
        best = callout.name;
        bestDistance = d;
      }
    }
    if (best !== '') return best;
  }
  return '';
}

/** The busiest cells of a layer, for the text list beside the radar. */
export function topCells(layer: HeatmapLayer, limit: number): HeatmapCell[] {
  return [...layer.cells]
    .sort((a, b) => b.count - a.count || a.seconds - b.seconds || a.y - b.y || a.x - b.x)
    .slice(0, limit);
}
