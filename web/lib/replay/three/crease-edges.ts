/**
 * The outline of the 3D map: only edges where two faces meet at a real fold,
 * such as a wall corner or the lip of a ledge. Open edges of the decimated
 * collision mesh and the seams of its tessellation are left out.
 */

export type CreaseEdges = {
  /** Line segment pairs, x,y,z per point. */
  positions: Float32Array;
  /** Per point: 1 when its segment runs mostly up and down, as on a wall corner. */
  vertical: Float32Array;
};

/** Vertices closer than this snap together, so faces split by the decimation still share edges. */
const WELD = 0.5;

type Edge = { a: number; b: number; normals: number[] };

/**
 * Crease edges of an unindexed triangle list. Winding is not trusted, so a
 * flipped neighbour on the same plane is not a crease.
 */
export function creaseEdges(positions: Float32Array, options: { angle: number; minLength: number }): CreaseEdges {
  const ids = new Map<string, number>();
  const points: number[] = [];
  const weld = (at: number): number => {
    const key = `${Math.round(positions[at] / WELD)},${Math.round(positions[at + 1] / WELD)},${Math.round(positions[at + 2] / WELD)}`;
    const found = ids.get(key);
    if (found !== undefined) return found;
    const id = points.length / 3;
    points.push(positions[at], positions[at + 1], positions[at + 2]);
    ids.set(key, id);
    return id;
  };
  const normals: number[] = [];
  const edges = new Map<number, Edge>();
  const span = positions.length;
  for (let t = 0; t + 8 < positions.length; t += 9) {
    const ux = positions[t + 3] - positions[t];
    const uy = positions[t + 4] - positions[t + 1];
    const uz = positions[t + 5] - positions[t + 2];
    const vx = positions[t + 6] - positions[t];
    const vy = positions[t + 7] - positions[t + 1];
    const vz = positions[t + 8] - positions[t + 2];
    const nx = uy * vz - uz * vy;
    const ny = uz * vx - ux * vz;
    const nz = ux * vy - uy * vx;
    const length = Math.hypot(nx, ny, nz);
    if (length === 0) continue;
    const face = normals.length / 3;
    normals.push(nx / length, ny / length, nz / length);
    const corners = [weld(t), weld(t + 3), weld(t + 6)];
    for (let k = 0; k < 3; k += 1) {
      const a = Math.min(corners[k], corners[(k + 1) % 3]);
      const b = Math.max(corners[k], corners[(k + 1) % 3]);
      if (a === b) continue;
      const key = a * span + b;
      const edge = edges.get(key);
      if (edge === undefined) edges.set(key, { a, b, normals: [face] });
      else edge.normals.push(face);
    }
  }
  const limit = Math.cos((options.angle * Math.PI) / 180);
  const out: number[] = [];
  const vertical: number[] = [];
  for (const { a, b, normals: faces } of edges.values()) {
    if (faces.length < 2 || !creased(normals, faces, limit)) continue;
    const dx = points[b * 3] - points[a * 3];
    const dy = points[b * 3 + 1] - points[a * 3 + 1];
    const dz = points[b * 3 + 2] - points[a * 3 + 2];
    const length = Math.hypot(dx, dy, dz);
    if (length < options.minLength) continue;
    out.push(points[a * 3], points[a * 3 + 1], points[a * 3 + 2], points[b * 3], points[b * 3 + 1], points[b * 3 + 2]);
    const up = Math.abs(dz) / length > 0.7 ? 1 : 0;
    vertical.push(up, up);
  }
  return { positions: Float32Array.from(out), vertical: Float32Array.from(vertical) };
}

function creased(normals: readonly number[], faces: readonly number[], limit: number): boolean {
  for (let i = 0; i < faces.length; i += 1) {
    for (let j = i + 1; j < faces.length; j += 1) {
      const p = faces[i] * 3;
      const q = faces[j] * 3;
      const dot = normals[p] * normals[q] + normals[p + 1] * normals[q + 1] + normals[p + 2] * normals[q + 2];
      if (Math.abs(dot) < limit) return true;
    }
  }
  return false;
}
