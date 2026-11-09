import type { Vec3 } from './vec3.ts';

/**
 * A bounding volume hierarchy over the map's collision triangles, flat in typed
 * arrays so a worker can build it and hand it over without a copy, and the map
 * can then cast a few hundred rays a frame against it.
 */

/**
 * `bounds` holds min x,y,z then max x,y,z per node. `links` holds two words per
 * node: a leaf has its first triangle and a count above 0; an inner node has
 * its right child and 0, and its left child right after it.
 */
export type SightBvh = { bounds: Float32Array; links: Uint32Array; triangles: Float32Array; nodeCount: number };

const LEAF_SIZE = 4;
const STACK_DEPTH = 96;
/** Hits closer than this to either end are the floor under a player's feet or the player's own hull. */
const END_SLACK = 1e-4;

type Build = {
  centroids: Float32Array;
  boxes: Float32Array;
  order: Uint32Array;
  bounds: Float32Array;
  links: Uint32Array;
  nodeCount: number;
};

function boxOf(build: Build, node: number, start: number, end: number): void {
  const at = node * 6;
  let minX = Infinity;
  let minY = Infinity;
  let minZ = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  let maxZ = -Infinity;
  for (let i = start; i < end; i += 1) {
    const box = build.order[i] * 6;
    minX = Math.min(minX, build.boxes[box]);
    minY = Math.min(minY, build.boxes[box + 1]);
    minZ = Math.min(minZ, build.boxes[box + 2]);
    maxX = Math.max(maxX, build.boxes[box + 3]);
    maxY = Math.max(maxY, build.boxes[box + 4]);
    maxZ = Math.max(maxZ, build.boxes[box + 5]);
  }
  build.bounds.set([minX, minY, minZ, maxX, maxY, maxZ], at);
}

/** The axis the range's centroids spread widest on. */
function widestAxis(build: Build, start: number, end: number): number {
  const lo = [Infinity, Infinity, Infinity];
  const hi = [-Infinity, -Infinity, -Infinity];
  for (let i = start; i < end; i += 1) {
    const c = build.order[i] * 3;
    for (let axis = 0; axis < 3; axis += 1) {
      lo[axis] = Math.min(lo[axis], build.centroids[c + axis]);
      hi[axis] = Math.max(hi[axis], build.centroids[c + axis]);
    }
  }
  const spread = [hi[0] - lo[0], hi[1] - lo[1], hi[2] - lo[2]];
  if (spread[0] >= spread[1] && spread[0] >= spread[2]) return 0;
  return spread[1] >= spread[2] ? 1 : 2;
}

function buildNode(build: Build, start: number, end: number): number {
  const node = build.nodeCount;
  build.nodeCount += 1;
  boxOf(build, node, start, end);
  if (end - start <= LEAF_SIZE) {
    build.links[node * 2] = start;
    build.links[node * 2 + 1] = end - start;
    return node;
  }
  const axis = widestAxis(build, start, end);
  const range = build.order.subarray(start, end);
  range.sort((a, b) => build.centroids[a * 3 + axis] - build.centroids[b * 3 + axis]);
  const middle = start + ((end - start) >> 1);
  buildNode(build, start, middle);
  build.links[node * 2] = buildNode(build, middle, end);
  build.links[node * 2 + 1] = 0;
  return node;
}

/** Builds the tree from a flat x,y,z vertex array and flat triangle indices. */
export function buildSightBvh(vertices: Float32Array, indices: Uint32Array): SightBvh {
  const count = Math.floor(indices.length / 3);
  const centroids = new Float32Array(count * 3);
  const boxes = new Float32Array(count * 6);
  for (let t = 0; t < count; t += 1) {
    for (let axis = 0; axis < 3; axis += 1) {
      const a = vertices[indices[t * 3] * 3 + axis];
      const b = vertices[indices[t * 3 + 1] * 3 + axis];
      const c = vertices[indices[t * 3 + 2] * 3 + axis];
      centroids[t * 3 + axis] = (a + b + c) / 3;
      boxes[t * 6 + axis] = Math.min(a, b, c);
      boxes[t * 6 + 3 + axis] = Math.max(a, b, c);
    }
  }
  const order = new Uint32Array(count);
  for (let t = 0; t < count; t += 1) order[t] = t;
  const maxNodes = Math.max(1, 2 * count);
  const build: Build = { centroids, boxes, order, bounds: new Float32Array(maxNodes * 6), links: new Uint32Array(maxNodes * 2), nodeCount: 0 };
  if (count > 0) buildNode(build, 0, count);
  const triangles = new Float32Array(count * 9);
  for (let i = 0; i < count; i += 1) {
    const t = order[i];
    for (let corner = 0; corner < 3; corner += 1) {
      const v = indices[t * 3 + corner] * 3;
      triangles.set([vertices[v], vertices[v + 1], vertices[v + 2]], i * 9 + corner * 3);
    }
  }
  return { bounds: build.bounds.slice(0, build.nodeCount * 6), links: build.links.slice(0, build.nodeCount * 2), triangles, nodeCount: build.nodeCount };
}

/** Ray parameter where the box is entered, or Infinity when the ray misses it before `limit`. */
function boxEntry(bvh: SightBvh, node: number, ray: Ray, limit: number): number {
  const b = node * 6;
  let near = 0;
  let far = limit;
  for (let axis = 0; axis < 3; axis += 1) {
    const inverse = ray.inverse[axis];
    let t0 = (bvh.bounds[b + axis] - ray.origin[axis]) * inverse;
    let t1 = (bvh.bounds[b + 3 + axis] - ray.origin[axis]) * inverse;
    if (Number.isNaN(t0) || Number.isNaN(t1)) continue;
    if (t0 > t1) [t0, t1] = [t1, t0];
    near = Math.max(near, t0);
    far = Math.min(far, t1);
    if (near > far) return Infinity;
  }
  return near;
}

type Ray = { origin: readonly [number, number, number]; direction: readonly [number, number, number]; inverse: readonly [number, number, number] };

/** Möller and Trumbore, both faces: the collision's winding is not uniform. */
function triangleHit(tris: Float32Array, t: number, ray: Ray): number {
  const i = t * 9;
  const e1x = tris[i + 3] - tris[i];
  const e1y = tris[i + 4] - tris[i + 1];
  const e1z = tris[i + 5] - tris[i + 2];
  const e2x = tris[i + 6] - tris[i];
  const e2y = tris[i + 7] - tris[i + 1];
  const e2z = tris[i + 8] - tris[i + 2];
  const [dx, dy, dz] = ray.direction;
  const px = dy * e2z - dz * e2y;
  const py = dz * e2x - dx * e2z;
  const pz = dx * e2y - dy * e2x;
  const det = e1x * px + e1y * py + e1z * pz;
  if (Math.abs(det) < 1e-9) return Infinity;
  const inv = 1 / det;
  const sx = ray.origin[0] - tris[i];
  const sy = ray.origin[1] - tris[i + 1];
  const sz = ray.origin[2] - tris[i + 2];
  const u = (sx * px + sy * py + sz * pz) * inv;
  if (u < 0 || u > 1) return Infinity;
  const qx = sy * e1z - sz * e1y;
  const qy = sz * e1x - sx * e1z;
  const qz = sx * e1y - sy * e1x;
  const v = (dx * qx + dy * qy + dz * qz) * inv;
  if (v < 0 || u + v > 1) return Infinity;
  return (e2x * qx + e2y * qy + e2z * qz) * inv;
}

const stack = new Uint32Array(STACK_DEPTH);

/** The nearest hit along `from + t * (to - from)` for t in (0, 1), or Infinity; `any` stops at the first. */
function trace(bvh: SightBvh, segment: { from: Vec3; to: Vec3; any: boolean }): number {
  if (bvh.nodeCount === 0) return Infinity;
  const { from, to } = segment;
  const direction: [number, number, number] = [to.x - from.x, to.y - from.y, to.z - from.z];
  const ray: Ray = { origin: [from.x, from.y, from.z], direction, inverse: [1 / direction[0], 1 / direction[1], 1 / direction[2]] };
  let best = 1 - END_SLACK;
  let found = Infinity;
  let top = 0;
  stack[top++] = 0;
  while (top > 0) {
    const node = stack[--top];
    if (boxEntry(bvh, node, ray, best) === Infinity) continue;
    const first = bvh.links[node * 2];
    const count = bvh.links[node * 2 + 1];
    if (count === 0) {
      if (top + 2 > STACK_DEPTH) continue;
      stack[top++] = first;
      stack[top++] = node + 1;
      continue;
    }
    for (let t = first; t < first + count; t += 1) {
      const at = triangleHit(bvh.triangles, t, ray);
      if (at > END_SLACK && at < best) {
        if (segment.any) return at;
        best = at;
        found = at;
      }
    }
  }
  return found;
}

/** True when collision stands between the two points. */
export function segmentBlocked(bvh: SightBvh, from: Vec3, to: Vec3): boolean {
  return trace(bvh, { from, to, any: true }) !== Infinity;
}

/** Share of the way from `from` to `to` where the first wall is, or 1 when the way is clear. */
export function clearShare(bvh: SightBvh, from: Vec3, to: Vec3): number {
  const at = trace(bvh, { from, to, any: false });
  return at === Infinity ? 1 : at;
}
