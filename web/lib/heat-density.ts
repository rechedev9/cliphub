import { heatRgb } from './tactical-spatial.ts';

// The heatmap as cs2lens paints it: the server's bins on a 150 grid over the radar, a Gaussian
// kernel per map times Suavizado, and filled contours at 24 thresholds. Picking stays on the cells.

/** Kernel width per map, in grid squares: tight maps get a narrower kernel. */
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

/** The Suavizado slider's range; 1 is the map's own bandwidth. */
export const SMOOTHING_MIN = 0.33;
export const SMOOTHING_MAX = 1.5;
export const SMOOTHING_DEFAULT = 1;

/** Contour thresholds between nothing and the peak. */
export const DENSITY_THRESHOLDS = 24;

/** Pixels of the picture per grid square, so the contour edges stay smooth on a large plate. */
const PICTURE_SCALE = 6;

/** The kernel's standard deviation in grid squares for a map and a smoothing factor. */
export function heatBandwidth(map: string, smoothing: number): number {
  const factor = Number.isFinite(smoothing) ? Math.min(SMOOTHING_MAX, Math.max(SMOOTHING_MIN, smoothing)) : SMOOTHING_DEFAULT;
  return (MAP_BANDWIDTH[map] ?? DEFAULT_BANDWIDTH) * factor;
}

/** A smoothed grid, row 0 at the radar's top. `tone` is the colour per square for timing, else absent. */
export type HeatDensity = { grid: number; mass: Float32Array; tone: Float32Array | null; peak: number };

export type DensityInput = {
  grid: number;
  bins: readonly (readonly [number, number, number])[];
  bandwidth: number;
  /** Each bin's own strength from 0 to 1, in `bins` order, smoothed by weight (timing); without it the colour follows the density. */
  tone?: readonly number[];
};

/** The kernel density of the bins, or null when there is nothing to paint. */
export function heatDensity({ grid, bins, bandwidth, tone }: DensityInput): HeatDensity | null {
  if (!(grid > 0) || bins.length === 0) return null;
  const mass = new Float32Array(grid * grid);
  const weighted = tone ? new Float32Array(grid * grid) : null;
  bins.forEach(([column, row, count], index) => {
    if (column < 0 || row < 0 || column >= grid || row >= grid || !(count > 0)) return;
    mass[row * grid + column] += count;
    if (weighted) weighted[row * grid + column] += count * (tone?.[index] ?? 0);
  });
  blur({ values: mass, grid, sigma: bandwidth });
  let toneOut: Float32Array | null = null;
  if (weighted) {
    blur({ values: weighted, grid, sigma: bandwidth });
    toneOut = new Float32Array(grid * grid);
    for (let i = 0; i < mass.length; i++) toneOut[i] = mass[i] > 1e-9 ? Math.min(1, Math.max(0, weighted[i] / mass[i])) : 0;
  }
  let peak = 0;
  for (const value of mass) peak = Math.max(peak, value);
  return peak > 0 ? { grid, mass, tone: toneOut, peak } : null;
}

/** The picture of a density: RGBA pixels over grid squares [x, x + width) and [y, y + height), `scale` pixels per square. */
export type DensityPicture = { rgba: Uint8ClampedArray<ArrayBuffer>; x: number; y: number; width: number; height: number; scale: number };

/** A reader of a grid at any point, bilinear between square centres and 0 off the grid. */
function sampler(values: Float32Array, grid: number): (gx: number, gy: number) => number {
  const at = (x: number, y: number): number => (x < 0 || y < 0 || x >= grid || y >= grid ? 0 : values[y * grid + x]);
  return (gx, gy) => {
    const x0 = Math.floor(gx - 0.5);
    const y0 = Math.floor(gy - 0.5);
    const tx = gx - 0.5 - x0;
    const ty = gy - 0.5 - y0;
    const top = at(x0, y0) * (1 - tx) + at(x0 + 1, y0) * tx;
    const bottom = at(x0, y0 + 1) * (1 - tx) + at(x0 + 1, y0 + 1) * tx;
    return top * (1 - ty) + bottom * ty;
  };
}

/** Filled contours of the density against `peak`; pass the larger peak of two floors so both read on one scale. */
export function densityPicture(density: HeatDensity, peak: number = density.peak): DensityPicture | null {
  const { grid, mass, tone } = density;
  if (!(peak > 0)) return null;
  const floor = peak / DENSITY_THRESHOLDS;
  let left = grid;
  let top = grid;
  let right = -1;
  let bottom = -1;
  for (let row = 0; row < grid; row++) {
    for (let column = 0; column < grid; column++) {
      if (mass[row * grid + column] < floor / 2) continue;
      left = Math.min(left, column);
      right = Math.max(right, column);
      top = Math.min(top, row);
      bottom = Math.max(bottom, row);
    }
  }
  if (right < 0) return null;
  const x = Math.max(0, left - 1);
  const y = Math.max(0, top - 1);
  const width = Math.min(grid, right + 2) - x;
  const height = Math.min(grid, bottom + 2) - y;
  const columns = width * PICTURE_SCALE;
  const rows = height * PICTURE_SCALE;
  const rgba = new Uint8ClampedArray(columns * rows * 4);
  const massAt = sampler(mass, grid);
  const toneAt = tone ? sampler(tone, grid) : null;
  for (let py = 0; py < rows; py++) {
    const gy = y + (py + 0.5) / PICTURE_SCALE;
    for (let px = 0; px < columns; px++) {
      const gx = x + (px + 0.5) / PICTURE_SCALE;
      // Thresholds at k/24 of the peak: under the first stays clear, over the last is the top band.
      const band = Math.min(DENSITY_THRESHOLDS - 1, Math.floor((massAt(gx, gy) / peak) * DENSITY_THRESHOLDS));
      if (band < 1) continue;
      const level = band / (DENSITY_THRESHOLDS - 1);
      // Timing colours each band by how early its players got there; the rest by the density itself.
      const strength = toneAt ? Math.round(toneAt(gx, gy) * DENSITY_THRESHOLDS) / DENSITY_THRESHOLDS : level;
      const [r, g, b] = heatRgb(strength);
      const i = (py * columns + px) * 4;
      rgba[i] = r;
      rgba[i + 1] = g;
      rgba[i + 2] = b;
      rgba[i + 3] = Math.round(255 * (0.3 + 0.55 * level));
    }
  }
  return { rgba, x, y, width, height, scale: PICTURE_SCALE };
}

/** A Gaussian blur in place, one axis then the other; the mass that leaves the grid is lost. */
function blur({ values, grid, sigma }: { values: Float32Array; grid: number; sigma: number }): void {
  if (!(sigma > 0)) return;
  const radius = Math.ceil(3 * sigma);
  const kernel = new Float32Array(2 * radius + 1);
  let sum = 0;
  for (let k = -radius; k <= radius; k++) {
    kernel[k + radius] = Math.exp(-(k * k) / (2 * sigma * sigma));
    sum += kernel[k + radius];
  }
  for (let k = 0; k < kernel.length; k++) kernel[k] /= sum;
  const temp = new Float32Array(values.length);
  for (let r = 0; r < grid; r++) {
    for (let c = 0; c < grid; c++) {
      let acc = 0;
      for (let k = -radius; k <= radius; k++) {
        const x = c + k;
        if (x >= 0 && x < grid) acc += values[r * grid + x] * kernel[k + radius];
      }
      temp[r * grid + c] = acc;
    }
  }
  for (let c = 0; c < grid; c++) {
    for (let r = 0; r < grid; r++) {
      let acc = 0;
      for (let k = -radius; k <= radius; k++) {
        const y = r + k;
        if (y >= 0 && y < grid) acc += temp[y * grid + c] * kernel[k + radius];
      }
      values[r * grid + c] = acc;
    }
  }
}
