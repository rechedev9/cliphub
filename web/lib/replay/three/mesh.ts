import { FLAT_NORMAL_Z } from './floor-field.ts';

/** Lowest and highest Z of a flat x,y,z vertex array; `[0, 0]` when empty. */
export function zRange(vertices: Float32Array): [number, number] {
  let min = Number.POSITIVE_INFINITY;
  let max = Number.NEGATIVE_INFINITY;
  for (let i = 2; i < vertices.length; i += 3) {
    const z = vertices[i];
    if (z < min) min = z;
    if (z > max) max = z;
  }
  return Number.isFinite(min) ? [min, max] : [0, 0];
}

type Rgb3 = readonly [number, number, number];

/** The floor tone at the lowest and the highest floor, sRGB 0..1, and how much darker walls sit. */
export type MapPalette = { low: Rgb3; high: Rgb3; wall: number };

/** Relative luminance of an sRGB colour, enough to tell a dark theme from a light one. */
function luminance(rgb: Rgb3): number {
  return 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
}

/**
 * The map's tones for the page behind it, the 2D esquema's greys lifted for
 * lighting: lower floors darker and cooler, higher floors lighter.
 */
export function mapPalette(background: Rgb3): MapPalette {
  return luminance(background) < 0.5
    ? { low: [0.3, 0.34, 0.41], high: [0.78, 0.79, 0.81], wall: 0.56 }
    : { low: [0.6, 0.63, 0.68], high: [0.9, 0.91, 0.93], wall: 0.7 };
}

const BANDS = 6;

/**
 * 0 at the lowest floor and 1 at the highest, eased like the esquema and
 * stepped into soft bands so each floor level reads as one tone.
 */
export function heightBand(z: number, low: number, high: number): number {
  const span = high - low;
  const t = span > 0 ? Math.sqrt(Math.min(1, Math.max(0, (z - low) / span))) : 0.5;
  const band = Math.min(BANDS - 1, Math.floor(t * BANDS)) / (BANDS - 1);
  return 0.7 * band + 0.3 * t;
}

/** sRGB channel to the linear value Three.js expects in vertex colours. */
export function srgbToLinear(channel: number): number {
  return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
}

/**
 * Per-vertex linear RGB for an unindexed triangle list: floors in their
 * height band, walls the band of their own height a step darker.
 */
export function surfaceColors(positions: Float32Array, palette: MapPalette, range: readonly [number, number]): Float32Array {
  const colors = new Float32Array(positions.length);
  for (let t = 0; t + 8 < positions.length; t += 9) {
    const ax = positions[t + 3] - positions[t];
    const ay = positions[t + 4] - positions[t + 1];
    const az = positions[t + 5] - positions[t + 2];
    const bx = positions[t + 6] - positions[t];
    const by = positions[t + 7] - positions[t + 1];
    const bz = positions[t + 8] - positions[t + 2];
    const nx = ay * bz - az * by;
    const ny = az * bx - ax * bz;
    const nz = ax * by - ay * bx;
    const length = Math.hypot(nx, ny, nz);
    const scale = length > 0 && Math.abs(nz / length) >= FLAT_NORMAL_Z ? 1 : palette.wall;
    for (let v = 0; v < 3; v += 1) {
      const at = t + v * 3;
      const tone = heightBand(positions[at + 2], range[0], range[1]);
      for (let c = 0; c < 3; c += 1) colors[at + c] = srgbToLinear((palette.low[c] + (palette.high[c] - palette.low[c]) * tone) * scale);
    }
  }
  return colors;
}
