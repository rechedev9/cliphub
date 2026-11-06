import test from 'node:test';
import assert from 'node:assert/strict';
import { buildSightBvh, clearShare, segmentBlocked } from './sight-bvh.ts';

/** A wall in the plane x = `x`, from y -100 to 100 and z 0 to 200, as two triangles. */
function wall(x: number): { vertices: number[]; indices: number[] } {
  return { vertices: [x, -100, 0, x, 100, 0, x, 100, 200, x, -100, 200], indices: [0, 1, 2, 0, 2, 3] };
}

function mesh(parts: { vertices: number[]; indices: number[] }[]): { vertices: Float32Array; indices: Uint32Array } {
  const vertices: number[] = [];
  const indices: number[] = [];
  for (const part of parts) {
    const base = vertices.length / 3;
    vertices.push(...part.vertices);
    indices.push(...part.indices.map((index) => index + base));
  }
  return { vertices: Float32Array.from(vertices), indices: Uint32Array.from(indices) };
}

test('sight bvh: a wall blocks the segment through it and not the one around it', () => {
  const { vertices, indices } = mesh([wall(0)]);
  const bvh = buildSightBvh(vertices, indices);
  assert.equal(segmentBlocked(bvh, { x: -200, y: 0, z: 64 }, { x: 200, y: 0, z: 64 }), true);
  assert.equal(segmentBlocked(bvh, { x: -200, y: 150, z: 64 }, { x: 200, y: 150, z: 64 }), false);
  assert.equal(segmentBlocked(bvh, { x: -200, y: 0, z: 264 }, { x: 200, y: 0, z: 264 }), false);
  assert.equal(segmentBlocked(bvh, { x: -200, y: 0, z: 64 }, { x: -50, y: 0, z: 64 }), false);
  assert.ok(Math.abs(clearShare(bvh, { x: -200, y: 0, z: 64 }, { x: 200, y: 0, z: 64 }) - 0.5) < 1e-6);
  assert.equal(clearShare(bvh, { x: -200, y: 150, z: 64 }, { x: 200, y: 150, z: 64 }), 1);
});

test('sight bvh: the tree answers like testing every triangle', () => {
  const parts = [];
  let seed = 7;
  const random = (): number => {
    seed = (seed * 1103515245 + 12345) % 2147483648;
    return seed / 2147483648;
  };
  for (let i = 0; i < 300; i += 1) {
    const x = random() * 4000 - 2000;
    const y = random() * 4000 - 2000;
    const size = 20 + random() * 120;
    parts.push({ vertices: [x, y, 0, x + size, y, random() * 200, x, y + size, 200], indices: [0, 1, 2] });
  }
  const { vertices, indices } = mesh(parts);
  const tree = buildSightBvh(vertices, indices);
  const flat = buildSightBvh(vertices, indices);
  assert.ok(tree.nodeCount > 50);
  // A one-node tree is the brute force answer: every triangle in a single leaf.
  const brute = { ...flat, nodeCount: 1, links: Uint32Array.from([0, indices.length / 3]), bounds: Float32Array.from([-1e9, -1e9, -1e9, 1e9, 1e9, 1e9]) };
  for (let i = 0; i < 400; i += 1) {
    const from = { x: random() * 4000 - 2000, y: random() * 4000 - 2000, z: random() * 200 };
    const to = { x: random() * 4000 - 2000, y: random() * 4000 - 2000, z: random() * 200 };
    assert.equal(segmentBlocked(tree, from, to), segmentBlocked(brute, from, to));
    assert.ok(Math.abs(clearShare(tree, from, to) - clearShare(brute, from, to)) < 1e-9);
  }
});

test('sight bvh: an empty mesh blocks nothing', () => {
  const bvh = buildSightBvh(new Float32Array(), new Uint32Array());
  assert.equal(segmentBlocked(bvh, { x: 0, y: 0, z: 0 }, { x: 100, y: 0, z: 0 }), false);
});
