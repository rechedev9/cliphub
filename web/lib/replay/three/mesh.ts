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

/**
 * Per-vertex RGB for the map: the base colour scaled by height, so lower floors
 * sit darker than the ones above them. `base` is linear RGB in 0..1.
 */
export function vertexColors(vertices: Float32Array, base: readonly [number, number, number], range: readonly [number, number]): Float32Array {
  const colors = new Float32Array(vertices.length);
  for (let i = 0; i < vertices.length; i += 3) {
    const shade = heightShade(vertices[i + 2], range[0], range[1]);
    colors[i] = base[0] * shade;
    colors[i + 1] = base[1] * shade;
    colors[i + 2] = base[2] * shade;
  }
  return colors;
}
