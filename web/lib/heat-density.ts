import type { HeatmapCell } from './api/spatial.ts';
import type { RadarCalibration } from './api/tactical.ts';
import type { WorldRect } from './tactical-transform.ts';
import { heatRgb } from './tactical-spatial.ts';

/**
 * The heatmap painted as a smooth density: each 64-unit cell's count spread
 * over a finer grid, then a Gaussian kernel estimate. The cells stay the data
 * for picking and the zone list; this is only how they look.
 */

/** Kernel width per map, in cells of a 150 cell grid over the radar square: tight maps get a narrower kernel. */
const MAP_BANDWIDTH: Record<string, number> = {
  de_nuke: 1.4,
  de_inferno: 2.1,
  de_mirage: 2.2,
  de_anubis: 2.2,
  de_overpass: 2.2,
  de_train: 2.2,
  de_vertigo: 2.2,
  de_dust2: 2.4,
  de_cache: 2.4,
  de_ancient: 2.6,
};
const DEFAULT_BANDWIDTH = 2.2;
const BANDWIDTH_GRID = 150;

/** The smoothing slider's range; 1 is the map's own bandwidth. */
export const SMOOTHING_MIN = 0.5;
export const SMOOTHING_MAX = 1.5;
export const SMOOTHING_DEFAULT = 1;

/** Largest grid side, so an outlier cell far from the rest cannot blow up the work. */
const MAX_GRID_SIDE = 640;

/** The kernel's standard deviation in world units for a map and a smoothing factor. */
export function heatSigma(calibration: RadarCalibration, smoothing: number): number {
  const bandwidth = MAP_BANDWIDTH[calibration.map] ?? DEFAULT_BANDWIDTH;
  const factor = Math.min(SMOOTHING_MAX, Math.max(SMOOTHING_MIN, Number.isFinite(smoothing) ? smoothing : SMOOTHING_DEFAULT));
  return (bandwidth * calibration.size * calibration.scale * factor) / BANDWIDTH_GRID;
}

/** A density grid, row 0 at the top (largest world y). `tone` is the colour strength per pixel for timing, else absent. */
export type HeatDensity = {
  cols: number;
  rows: number;
  minX: number;
  maxY: number;
  step: number;
  mass: Float32Array;
  tone: Float32Array | null;
  peak: number;
};

export type DensityInput = {
  cells: readonly HeatmapCell[];
  cellSize: number;
  sigma: number;
  /** A cell's own strength from 0 to 1, smoothed by weight (timing); without it the colour follows the density. */
  tone?: (cell: HeatmapCell) => number;
};

/** The density of the cells, or null when there is nothing to paint. */
export function heatDensity({ cells, cellSize, sigma, tone }: DensityInput): HeatDensity | null {
  if (cells.length === 0 || !(cellSize > 0) || !(sigma > 0)) return null;
  let lowX = Infinity;
  let lowY = Infinity;
  let highX = -Infinity;
  let highY = -Infinity;
  for (const cell of cells) {
    lowX = Math.min(lowX, cell.x);
    lowY = Math.min(lowY, cell.y);
    highX = Math.max(highX, cell.x + 1);
    highY = Math.max(highY, cell.y + 1);
  }
  const span = Math.max(highX - lowX, highY - lowY) * cellSize + 6 * sigma;
  const step = Math.max(cellSize / 4, span / MAX_GRID_SIDE);
  const pad = Math.ceil((3 * sigma) / step + 1) * step;
  const minX = lowX * cellSize - pad;
  const maxY = highY * cellSize + pad;
  const cols = Math.max(1, Math.ceil(((highX - lowX) * cellSize + 2 * pad) / step));
  const rows = Math.max(1, Math.ceil(((highY - lowY) * cellSize + 2 * pad) / step));
  const mass = new Float32Array(cols * rows);
  const weighted = tone ? new Float32Array(cols * rows) : null;
  for (const cell of cells) {
    if (!(cell.count > 0)) continue;
    const c0 = clamp(Math.floor((cell.x * cellSize - minX) / step), 0, cols - 1);
    const c1 = clamp(Math.ceil(((cell.x + 1) * cellSize - minX) / step) - 1, c0, cols - 1);
    const r0 = clamp(Math.floor((maxY - (cell.y + 1) * cellSize) / step), 0, rows - 1);
    const r1 = clamp(Math.ceil((maxY - cell.y * cellSize) / step) - 1, r0, rows - 1);
    const share = cell.count / ((c1 - c0 + 1) * (r1 - r0 + 1));
    const strength = tone ? tone(cell) : 0;
    for (let r = r0; r <= r1; r++) {
      for (let c = c0; c <= c1; c++) {
        mass[r * cols + c] += share;
        if (weighted) weighted[r * cols + c] += share * strength;
      }
    }
  }
  const grid = { cols, rows };
  blur(mass, grid, sigma / step);
  let toneOut: Float32Array | null = null;
  if (weighted) {
    blur(weighted, grid, sigma / step);
    toneOut = new Float32Array(cols * rows);
    for (let i = 0; i < mass.length; i++) toneOut[i] = mass[i] > 1e-9 ? Math.min(1, Math.max(0, weighted[i] / mass[i])) : 0;
  }
  let peak = 0;
  for (const value of mass) peak = Math.max(peak, value);
  return { cols, rows, minX, maxY, step, mass, tone: toneOut, peak };
}

/** The world rectangle the grid covers, to place its image on the radar. */
export function densityRect(grid: HeatDensity): WorldRect {
  return { minX: grid.minX, minY: grid.maxY - grid.rows * grid.step, maxX: grid.minX + grid.cols * grid.step, maxY: grid.maxY };
}

/**
 * RGBA pixels of the grid on the heat scale, read against `peak` (pass the
 * larger peak of two floors so both share one scale). The faint tail fades
 * out and the strongest spots stay translucent, so the map reads underneath.
 */
export function densityRgba(grid: HeatDensity, peak: number = grid.peak): Uint8ClampedArray<ArrayBuffer> {
  const out = new Uint8ClampedArray(grid.cols * grid.rows * 4);
  if (!(peak > 0)) return out;
  for (let i = 0; i < grid.mass.length; i++) {
    const density = Math.min(1, grid.mass[i] / peak);
    const edge = smoothstep(0.01, 0.08, density);
    if (edge <= 0) continue;
    // Counts are long-tailed, so the colour ramps on the square root, like the cells did.
    const strength = grid.tone ? grid.tone[i] : Math.sqrt(density);
    const [r, g, b] = heatRgb(strength);
    out[i * 4] = r;
    out[i * 4 + 1] = g;
    out[i * 4 + 2] = b;
    out[i * 4 + 3] = Math.round(255 * edge * (0.35 + 0.5 * Math.sqrt(density)));
  }
  return out;
}

function blur(values: Float32Array, grid: { cols: number; rows: number }, sigma: number): void {
  if (!(sigma >= 0.5)) return;
  const radius = Math.ceil(3 * sigma);
  const kernel = new Float32Array(2 * radius + 1);
  let sum = 0;
  for (let k = -radius; k <= radius; k++) {
    kernel[k + radius] = Math.exp(-(k * k) / (2 * sigma * sigma));
    sum += kernel[k + radius];
  }
  for (let k = 0; k < kernel.length; k++) kernel[k] /= sum;
  const { cols, rows } = grid;
  const temp = new Float32Array(values.length);
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      let acc = 0;
      for (let k = -radius; k <= radius; k++) {
        const x = c + k;
        if (x >= 0 && x < cols) acc += values[r * cols + x] * kernel[k + radius];
      }
      temp[r * cols + c] = acc;
    }
  }
  for (let c = 0; c < cols; c++) {
    for (let r = 0; r < rows; r++) {
      let acc = 0;
      for (let k = -radius; k <= radius; k++) {
        const y = r + k;
        if (y >= 0 && y < rows) acc += temp[y * cols + c] * kernel[k + radius];
      }
      values[r * cols + c] = acc;
    }
  }
}

function clamp(value: number, low: number, high: number): number {
  return Math.min(high, Math.max(low, value));
}

function smoothstep(edge0: number, edge1: number, x: number): number {
  const t = clamp((x - edge0) / (edge1 - edge0), 0, 1);
  return t * t * (3 - 2 * t);
}
