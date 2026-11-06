import test from 'node:test';
import assert from 'node:assert/strict';
import { buildFloorField, floorAt, markWalls } from './floor-field.ts';

/** Two nav quads side by side: a street at z 0 on x 0..256 and a ledge at z 128 on x 256..512. */
function nav(): { vertices: Float32Array; triangles: Uint32Array } {
  const vertices = new Float32Array([
    0, 0, 0, 256, 0, 0, 256, 256, 0, 0, 256, 0,
    256, 0, 128, 512, 0, 128, 512, 256, 128, 256, 256, 128,
  ]);
  const triangles = new Uint32Array([0, 1, 2, 0, 2, 3, 4, 5, 6, 4, 6, 7]);
  return { vertices, triangles };
}

const bounds = { minX: -256, minY: -256, maxX: 768, maxY: 512 };

test('floor field: each cell holds the floor under it and empty cells take the nearest floor', () => {
  const field = buildFloorField(nav(), bounds, 32);
  assert.ok(field !== null);
  assert.equal(floorAt(field, 100, 100), 0);
  assert.equal(floorAt(field, 400, 100), 128);
  // Far outside the nav, past the ledge: the ledge is the nearest floor.
  assert.equal(floorAt(field, 760, 100), 128);
  assert.equal(floorAt(field, -250, 100), 0);
  assert.deepEqual([field.lowZ, field.highZ], [0, 128]);
  const cell = (x: number, y: number): number => Math.floor((y - field.minY) / field.cell) * field.cols + Math.floor((x - field.minX) / field.cell);
  assert.equal(field.outside[cell(100, 100)], 0);
  assert.equal(field.outside[cell(540, 100)], 0, 'the wall line beside the floor is still the map');
  assert.ok(field.outside[cell(760, 100)] > 0.5, 'far past the nav is outside');
});

test('floor field: a stacked floor keeps the higher one, so a bridge is never cut over its own deck', () => {
  const stacked = nav();
  const bridge = new Float32Array([...stacked.vertices, 0, 0, 200, 256, 0, 200, 256, 256, 200]);
  const field = buildFloorField({ vertices: bridge, triangles: new Uint32Array([...stacked.triangles, 8, 9, 10]) }, bounds, 32);
  assert.ok(field !== null);
  assert.equal(floorAt(field, 200, 50), 200);
});

test('floor field: no nav, no field', () => {
  assert.equal(buildFloorField({ vertices: new Float32Array(), triangles: new Uint32Array() }, bounds, 32), null);
});

test('floor field: floors beside the foot of a wall darken, open floor and wall tops above the foot do not', () => {
  const field = buildFloorField(nav(), bounds, 32);
  assert.ok(field !== null);
  // A wall along y = 0 on the street, 0 to 300 high, and a free-standing slab far above the floor.
  const wall = new Float32Array([0, 0, 0, 256, 0, 0, 256, 0, 300, 0, 0, 0, 256, 0, 300, 0, 0, 300]);
  markWalls(field, wall);
  const near = field.walls[Math.floor((16 - field.minY) / field.cell) * field.cols + Math.floor((128 - field.minX) / field.cell)];
  const open = field.walls[Math.floor((200 - field.minY) / field.cell) * field.cols + Math.floor((128 - field.minX) / field.cell)];
  assert.ok(near > 0.5, `near ${near}`);
  assert.equal(open, 0);
});
