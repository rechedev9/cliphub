import { heightShade } from './scene-math.ts';

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

/** Floor and wall tones of the 3D map; linear RGB in 0..1. */
export type MapPalette = { floor: Rgb3; wall: Rgb3 };

/** Relative luminance of an sRGB colour, enough to tell a dark theme from a light one. */
function luminance(rgb: Rgb3): number {
  return 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2];
}

/**
 * The map's two tones for the page behind it: on a dark page light concrete
 * floors over darker walls, on a light page the same family one step lighter,
 * so the map always stands off its background.
 */
export function mapPalette(background: Rgb3): MapPalette {
  return luminance(background) < 0.5
    ? { floor: [0.5, 0.52, 0.55], wall: [0.24, 0.26, 0.3] }
    : { floor: [0.72, 0.73, 0.75], wall: [0.42, 0.44, 0.49] };
}

/** A triangle whose normal leans less than this from straight up is floor. */
const FLOOR_NORMAL_Z = 0.7;

/**
 * Per-vertex RGB for an unindexed triangle list: floors in the floor tone and
 * walls in the wall tone, both scaled by height so lower floors and the foot of
 * every wall sit darker, a cheap stand-in for ambient occlusion.
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
    const floor = length > 0 && Math.abs(nz / length) >= FLOOR_NORMAL_Z;
    const base = floor ? palette.floor : palette.wall;
    for (let v = 0; v < 3; v += 1) {
      const at = t + v * 3;
      const shade = heightShade(positions[at + 2], range[0], range[1]);
      colors[at] = base[0] * shade;
      colors[at + 1] = base[1] * shade;
      colors[at + 2] = base[2] * shade;
    }
  }
  return colors;
}
