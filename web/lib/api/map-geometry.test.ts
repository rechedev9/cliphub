import test from 'node:test';
import assert from 'node:assert/strict';
import { MAP_GEOMETRY_NAME_RE, parseMapGeometry } from './map-geometry.ts';

const mesh = {
  map: 'de_test',
  source: 'nav',
  build: '2000942',
  bounds: { min: [0, 0, 10], max: [100, 100, 10] },
  vertices: [0, 0, 10, 100, 0, 10, 100, 100, 10],
  triangles: [0, 1, 2],
  areas: [{ id: 7, polygon: [0, 1, 2] }],
};

test('parseMapGeometry returns typed buffers ready for a BufferGeometry', () => {
  const parsed = parseMapGeometry(mesh);
  assert.ok(parsed);
  assert.ok(parsed.vertices instanceof Float32Array);
  assert.deepEqual(Array.from(parsed.triangles), [0, 1, 2]);
  assert.deepEqual(parsed.bounds, { min: [0, 0, 10], max: [100, 100, 10] });
  assert.deepEqual(parsed.areas, [{ id: 7, polygon: [0, 1, 2] }]);
});

test('parseMapGeometry rejects indices past the vertex list and broken shapes', () => {
  assert.equal(parseMapGeometry({ ...mesh, triangles: [0, 1, 3] }), null);
  assert.equal(parseMapGeometry({ ...mesh, areas: [{ id: 7, polygon: [0, 9] }] }), null);
  assert.equal(parseMapGeometry({ ...mesh, vertices: [0, 0] }), null);
  assert.equal(parseMapGeometry({ ...mesh, source: 'physics' }), null);
  assert.equal(parseMapGeometry({ ...mesh, bounds: { min: [0, 0], max: [1, 1, 1] } }), null);
});

test('MAP_GEOMETRY_NAME_RE admits CS2 map names only', () => {
  for (const ok of ['de_mirage', 'cs_office', 'ar_baggage', 'de_dust2']) assert.ok(MAP_GEOMETRY_NAME_RE.test(ok), ok);
  for (const bad of ['DE_MIRAGE', 'de-mirage', '../de_mirage', 'de_', `de_${'a'.repeat(33)}`, 'workshop_x']) {
    assert.ok(!MAP_GEOMETRY_NAME_RE.test(bad), bad);
  }
});
