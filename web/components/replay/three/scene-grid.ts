import { ClampToEdgeWrapping, Data3DTexture, DataTexture, FloatType, NearestFilter, RGBAFormat, UnsignedByteType, Vector3 } from 'three';
import type { MapScene } from '@/lib/api/map-scene';

/** Items a grid cell lists, in the order given; a fragment tests each one it lists. */
export const CELL_ITEMS = 4;
/** The grid's finest cell and most cells per axis. */
const MIN_CELL = 32;
const MAX_CELLS = 192;

export type Box = { lo: number[]; hi: number[] };
export type BoxGrid = { texture: Data3DTexture; min: Vector3; scale: Vector3 };

/**
 * A 3D grid over the map whose RGBA cells hold, plus one, the indices of the
 * first `CELL_ITEMS` boxes that reach them, in the order of `boxes`; 0 is none.
 */
export function boxGrid(scene: MapScene, boxes: Box[]): BoxGrid {
  const min = scene.min.map((v) => v - 64);
  const extent = scene.max.map((v, i) => v + 64 - min[i]);
  const cell = Math.max(MIN_CELL, Math.max(...extent) / MAX_CELLS);
  const dims = extent.map((e) => Math.max(1, Math.min(MAX_CELLS, Math.ceil(e / cell))));
  const [nx, ny, nz] = dims;
  const data = new Uint8Array(nx * ny * nz * CELL_ITEMS);
  const size = extent.map((e, i) => e / dims[i]);
  boxes.slice(0, 255).forEach(({ lo, hi }, index) => {
    const from = lo.map((v, i) => Math.max(0, Math.floor((v - min[i]) / size[i])));
    const to = hi.map((v, i) => Math.min(dims[i] - 1, Math.floor((v - min[i]) / size[i])));
    for (let z = from[2]; z <= to[2]; z++) {
      for (let y = from[1]; y <= to[1]; y++) {
        for (let x = from[0]; x <= to[0]; x++) {
          const at = ((z * ny + y) * nx + x) * CELL_ITEMS;
          const free = data.subarray(at, at + CELL_ITEMS).indexOf(0);
          if (free >= 0) data[at + free] = index + 1;
        }
      }
    }
  });
  const texture = new Data3DTexture(data, nx, ny, nz);
  texture.format = RGBAFormat;
  texture.type = UnsignedByteType;
  texture.minFilter = NearestFilter;
  texture.magFilter = NearestFilter;
  texture.wrapS = ClampToEdgeWrapping;
  texture.wrapT = ClampToEdgeWrapping;
  texture.wrapR = ClampToEdgeWrapping;
  texture.unpackAlignment = 1;
  texture.needsUpdate = true;
  return { texture, min: new Vector3(min[0], min[1], min[2]), scale: new Vector3(1 / extent[0], 1 / extent[1], 1 / extent[2]) };
}

/** A float RGBA table with one row per item of `texels` texels, each filled by `fill` at its row's offset. */
export function rowTable<T>(items: T[], texels: number, fill: (data: Float32Array, row: number, item: T) => void): DataTexture {
  const data = new Float32Array(texels * 4 * Math.max(1, items.length));
  items.forEach((item, i) => fill(data, i * texels * 4, item));
  const texture = new DataTexture(data, texels, Math.max(1, items.length), RGBAFormat, FloatType);
  texture.minFilter = NearestFilter;
  texture.magFilter = NearestFilter;
  texture.needsUpdate = true;
  return texture;
}
