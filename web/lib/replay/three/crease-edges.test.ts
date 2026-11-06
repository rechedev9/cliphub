import test from 'node:test';
import assert from 'node:assert/strict';
import { creaseEdges } from './crease-edges.ts';

test('crease edges: a floor meeting a wall leaves one line, the seam of a flat floor none', () => {
  // A floor of two triangles (one wound the other way) and a wall standing on its far edge.
  const positions = new Float32Array([
    0, 0, 0, 100, 0, 0, 100, 100, 0,
    0, 0, 0, 0, 100, 0, 100, 100, 0,
    100, 0, 0, 100, 100, 0, 100, 100, 100,
  ]);
  const edges = creaseEdges(positions, { angle: 40, minLength: 1 });
  assert.equal(edges.positions.length, 6);
  const xs = [edges.positions[0], edges.positions[3]];
  assert.deepEqual(xs, [100, 100]);
  assert.deepEqual([...edges.vertical], [0, 0]);
});

test('crease edges: a wall corner is vertical and edges shorter than the minimum go', () => {
  const positions = new Float32Array([
    0, 0, 0, 100, 0, 0, 0, 0, 100,
    0, 0, 0, 0, 100, 0, 0, 0, 100,
  ]);
  const edges = creaseEdges(positions, { angle: 40, minLength: 1 });
  assert.deepEqual([...edges.vertical], [1, 1]);
  assert.equal(creaseEdges(positions, { angle: 40, minLength: 150 }).positions.length, 0);
});
