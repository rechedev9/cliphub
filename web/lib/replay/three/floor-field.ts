/**
 * A coarse height field of the walkable floor, read from the nav mesh: one Z
 * per square cell, the highest floor in it. The 3D map cuts walls and roofs
 * against it, so a roof goes wherever it hangs over a floor, at any height.
 */

export type FloorField = {
  minX: number;
  minY: number;
  cell: number;
  cols: number;
  rows: number;
  /** Floor Z per cell, row-major from `minY`; empty cells take the nearest floor. */
  floor: Float32Array;
  /** 0..1 per cell: how much wall stands at the foot of this floor, for ambient occlusion. */
  walls: Float32Array;
  /** 0..1 per cell: 0 on and beside the walkable floor, 1 well outside the playable map. */
  outside: Float32Array;
  /** The floor heights the tone spans, with stray areas trimmed off both ends. */
  lowZ: number;
  highZ: number;
};

export type Bounds2 = { minX: number; minY: number; maxX: number; maxY: number };

/** Cells per side never exceed this, so a huge map gets coarser cells instead of a huge texture. */
const MAX_CELLS = 512;
/** Share of floor cells ignored at each end of the tone range. */
const TONE_TRIM = 0.02;
/** A face whose normal leans less than this from vertical is floor or roof. */
export const FLAT_NORMAL_Z = 0.7;

function quantile(sorted: Float32Array, q: number): number {
  if (sorted.length === 0) return 0;
  return sorted[Math.min(sorted.length - 1, Math.max(0, Math.round(q * (sorted.length - 1))))];
}

function emptyField(bounds: Bounds2, cellSize: number): FloorField {
  const spanX = Math.max(1, bounds.maxX - bounds.minX);
  const spanY = Math.max(1, bounds.maxY - bounds.minY);
  const cell = Math.max(cellSize, spanX / MAX_CELLS, spanY / MAX_CELLS);
  const cols = Math.ceil(spanX / cell) + 1;
  const rows = Math.ceil(spanY / cell) + 1;
  return {
    minX: bounds.minX,
    minY: bounds.minY,
    cell,
    cols,
    rows,
    floor: new Float32Array(cols * rows).fill(Number.NaN),
    walls: new Float32Array(cols * rows),
    outside: new Float32Array(cols * rows),
    lowZ: 0,
    highZ: 0,
  };
}

function cellIndex(field: FloorField, x: number, y: number): number {
  const col = Math.min(field.cols - 1, Math.max(0, Math.floor((x - field.minX) / field.cell)));
  const row = Math.min(field.rows - 1, Math.max(0, Math.floor((y - field.minY) / field.cell)));
  return row * field.cols + col;
}

function raise(field: FloorField, index: number, z: number): void {
  const current = field.floor[index];
  if (Number.isNaN(current) || z > current) field.floor[index] = z;
}

type Corner = { x: number; y: number; z: number };

/** Writes the triangle's height at every cell centre it covers, and at its corners for thin slivers. */
function rasterize(field: FloorField, a: Corner, b: Corner, c: Corner): void {
  const det = (b.y - c.y) * (a.x - c.x) + (c.x - b.x) * (a.y - c.y);
  for (const corner of [a, b, c]) raise(field, cellIndex(field, corner.x, corner.y), corner.z);
  raise(field, cellIndex(field, (a.x + b.x + c.x) / 3, (a.y + b.y + c.y) / 3), (a.z + b.z + c.z) / 3);
  if (Math.abs(det) < 1e-6) return;
  const { cell, minX, minY } = field;
  const col0 = Math.max(0, Math.floor((Math.min(a.x, b.x, c.x) - minX) / cell));
  const col1 = Math.min(field.cols - 1, Math.floor((Math.max(a.x, b.x, c.x) - minX) / cell));
  const row0 = Math.max(0, Math.floor((Math.min(a.y, b.y, c.y) - minY) / cell));
  const row1 = Math.min(field.rows - 1, Math.floor((Math.max(a.y, b.y, c.y) - minY) / cell));
  for (let row = row0; row <= row1; row += 1) {
    const y = minY + (row + 0.5) * cell;
    for (let col = col0; col <= col1; col += 1) {
      const x = minX + (col + 0.5) * cell;
      const wa = ((b.y - c.y) * (x - c.x) + (c.x - b.x) * (y - c.y)) / det;
      const wb = ((c.y - a.y) * (x - c.x) + (a.x - c.x) * (y - c.y)) / det;
      const wc = 1 - wa - wb;
      if (wa < -1e-6 || wb < -1e-6 || wc < -1e-6) continue;
      raise(field, row * field.cols + col, wa * a.z + wb * b.z + wc * c.z);
    }
  }
}

/** Cells this many steps from the nav still count as the map: the walls and props round the floor. */
const OUTSIDE_FROM = 3;
const OUTSIDE_RAMP = 6;

/** Gives every empty cell the floor of its nearest filled cell, breadth first, and marks how far outside the map it lies. */
function fillEmpty(field: FloorField): void {
  const steps = new Int32Array(field.floor.length);
  const queue = new Int32Array(field.floor.length);
  let head = 0;
  let tail = 0;
  for (let i = 0; i < field.floor.length; i += 1) if (!Number.isNaN(field.floor[i])) queue[tail++] = i;
  while (head < tail) {
    const at = queue[head++];
    const col = at % field.cols;
    const z = field.floor[at];
    const neighbours = [col > 0 ? at - 1 : -1, col < field.cols - 1 ? at + 1 : -1, at - field.cols, at + field.cols];
    for (const next of neighbours) {
      if (next < 0 || next >= field.floor.length || !Number.isNaN(field.floor[next])) continue;
      field.floor[next] = z;
      steps[next] = steps[at] + 1;
      field.outside[next] = Math.min(1, Math.max(0, (steps[next] - OUTSIDE_FROM) / OUTSIDE_RAMP));
      queue[tail++] = next;
    }
  }
}

/**
 * The floor field of a nav mesh over `bounds`, or null when the nav has no
 * triangles. `cell` is the cell side in world units.
 */
export function buildFloorField(nav: { vertices: Float32Array; triangles: Uint32Array }, bounds: Bounds2, cell: number): FloorField | null {
  if (nav.triangles.length < 3) return null;
  const field = emptyField(bounds, cell);
  const v = nav.vertices;
  const corner = (index: number): Corner => ({ x: v[index * 3], y: v[index * 3 + 1], z: v[index * 3 + 2] });
  for (let t = 0; t + 2 < nav.triangles.length; t += 3) {
    rasterize(field, corner(nav.triangles[t]), corner(nav.triangles[t + 1]), corner(nav.triangles[t + 2]));
  }
  const filled = field.floor.filter((z) => !Number.isNaN(z)).sort();
  if (filled.length === 0) return null;
  field.lowZ = quantile(filled, TONE_TRIM);
  field.highZ = quantile(filled, 1 - TONE_TRIM);
  fillEmpty(field);
  return field;
}

/** The floor under a point, bilinear between cell centres as the GPU samples it. */
export function floorAt(field: FloorField, x: number, y: number): number {
  const fx = Math.min(field.cols - 1, Math.max(0, (x - field.minX) / field.cell - 0.5));
  const fy = Math.min(field.rows - 1, Math.max(0, (y - field.minY) / field.cell - 0.5));
  const c0 = Math.floor(fx);
  const r0 = Math.floor(fy);
  const c1 = Math.min(field.cols - 1, c0 + 1);
  const r1 = Math.min(field.rows - 1, r0 + 1);
  const tx = fx - c0;
  const ty = fy - r0;
  const at = (col: number, row: number): number => field.floor[row * field.cols + col];
  const bottom = at(c0, r0) + (at(c1, r0) - at(c0, r0)) * tx;
  const top = at(c0, r1) + (at(c1, r1) - at(c0, r1)) * tx;
  return bottom + (top - bottom) * ty;
}

/** Wall within this height over the floor darkens the floor beside it. */
const WALL_FOOT = 96;
const MAX_SAMPLES_PER_SIDE = 48;

/** Box blur of one cell radius, in place. */
function blur(field: FloorField, values: Float32Array): void {
  const copy = Float32Array.from(values);
  for (let row = 0; row < field.rows; row += 1) {
    for (let col = 0; col < field.cols; col += 1) {
      let sum = 0;
      let count = 0;
      for (let dr = -1; dr <= 1; dr += 1) {
        for (let dc = -1; dc <= 1; dc += 1) {
          const r = row + dr;
          const c = col + dc;
          if (r < 0 || c < 0 || r >= field.rows || c >= field.cols) continue;
          sum += copy[r * field.cols + c];
          count += 1;
        }
      }
      values[row * field.cols + col] = sum / count;
    }
  }
}

/**
 * Fills `field.walls` from an unindexed triangle list: cells where a wall
 * rises from the floor score high, softened so corners, with walls on two
 * sides, end up darkest.
 */
export function markWalls(field: FloorField, positions: Float32Array): void {
  const hits = new Float32Array(field.walls.length);
  for (let t = 0; t + 8 < positions.length; t += 9) {
    const ax = positions[t];
    const ay = positions[t + 1];
    const az = positions[t + 2];
    const ux = positions[t + 3] - ax;
    const uy = positions[t + 4] - ay;
    const uz = positions[t + 5] - az;
    const vx = positions[t + 6] - ax;
    const vy = positions[t + 7] - ay;
    const vz = positions[t + 8] - az;
    const nz = ux * vy - uy * vx;
    const length = Math.hypot(uy * vz - uz * vy, uz * vx - ux * vz, nz);
    if (length === 0 || Math.abs(nz / length) >= FLAT_NORMAL_Z) continue;
    const longest = Math.max(Math.hypot(ux, uy, uz), Math.hypot(vx, vy, vz), Math.hypot(vx - ux, vy - uy, vz - uz));
    const steps = Math.min(MAX_SAMPLES_PER_SIDE, Math.max(1, Math.ceil(longest / (field.cell / 2))));
    for (let i = 0; i <= steps; i += 1) {
      for (let j = 0; i + j <= steps; j += 1) {
        const s = i / steps;
        const r = j / steps;
        const x = ax + ux * s + vx * r;
        const y = ay + uy * s + vy * r;
        const rise = az + uz * s + vz * r - field.floor[cellIndex(field, x, y)];
        if (rise > -16 && rise < WALL_FOOT) hits[cellIndex(field, x, y)] = 1;
      }
    }
  }
  blur(field, hits);
  blur(field, hits);
  for (let i = 0; i < hits.length; i += 1) field.walls[i] = Math.min(1, hits[i] * 1.6);
}
