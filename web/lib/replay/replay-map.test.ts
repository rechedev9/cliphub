import test from 'node:test';
import assert from 'node:assert/strict';
import { FOLLOW_ZOOM, MAX_ZOOM, followView, panView, zoomLevel, zoomViewAt } from './map-view.ts';
import { heightTone, navRadarModel, polygonLevel, toneColor } from './nav-radar.ts';
import { layoutTags, tagName } from './tag-layout.ts';

const base = { x: 0.1, y: 0.1, width: 0.8, height: 0.8 };
const box = { width: 800, height: 800 };

test('map view: zooming keeps the point under the cursor in place', () => {
  const cursor = { x: 200, y: 600 };
  const view = zoomViewAt({ box, view: base, base, cursor, factor: 2 });
  assert.equal(zoomLevel(view, base), 2);
  // The square point under the cursor before and after.
  const before = { x: base.x + (cursor.x / 800) * base.width, y: base.y + (cursor.y / 800) * base.height };
  const after = { x: view.x + (cursor.x / 800) * view.width, y: view.y + (cursor.y / 800) * view.height };
  assert.ok(Math.abs(before.x - after.x) < 1e-9);
  assert.ok(Math.abs(before.y - after.y) < 1e-9);
});

test('map view: zoom stays between the whole map and the cap', () => {
  assert.deepEqual(zoomViewAt({ box, view: base, base, cursor: { x: 400, y: 400 }, factor: 0.5 }), base);
  const deep = zoomViewAt({ box, view: base, base, cursor: { x: 400, y: 400 }, factor: 100 });
  assert.equal(zoomLevel(deep, base), MAX_ZOOM);
});

test('map view: a drag pans a zoomed map and never past its edge', () => {
  const zoomed = zoomViewAt({ box, view: base, base, cursor: { x: 400, y: 400 }, factor: 2 });
  const moved = panView({ box, view: zoomed, base, dx: 100, dy: 0 });
  assert.ok(moved.x < zoomed.x);
  const far = panView({ box, view: zoomed, base, dx: 100000, dy: 0 });
  assert.equal(far.x + far.width / 2, base.x);
  assert.deepEqual(panView({ box, view: base, base, dx: 50, dy: 50 }), base);
});

test('nav radar: areas become polygons sorted by height with a trimmed range', () => {
  const vertices = Float32Array.from([0, 0, 100, 10, 0, 100, 10, 10, 100, 0, 0, 0, 10, 0, 0, 10, 10, 0]);
  const model = navRadarModel({ vertices, triangles: new Uint32Array(), areas: [{ id: 1, polygon: [0, 1, 2] }, { id: 2, polygon: [3, 4, 5] }] });
  assert.ok(model !== null);
  assert.deepEqual(model.polygons.map((polygon) => polygon.z), [0, 100]);
  assert.deepEqual(model.bounds, { minX: 0, minY: 0, maxX: 10, maxY: 10 });
  assert.equal(navRadarModel({ vertices: new Float32Array(), triangles: new Uint32Array(), areas: [] }), null);
});

test('nav radar: without areas the triangles are the polygons', () => {
  const vertices = Float32Array.from([0, 0, 5, 1, 0, 5, 0, 1, 5]);
  const model = navRadarModel({ vertices, triangles: Uint32Array.from([0, 1, 2]), areas: [] });
  assert.equal(model?.polygons.length, 1);
});

test('nav radar: higher floors are lighter, flat maps sit mid-grey', () => {
  assert.equal(heightTone(0, 0, 100), 0);
  assert.equal(heightTone(100, 0, 100), 1);
  assert.ok(heightTone(25, 0, 100) > 0.25);
  assert.equal(heightTone(5, 5, 5), 0.5);
  assert.equal(toneColor(0, [0, 0, 0], [200, 100, 50]), 'rgb(0, 0, 0)');
  assert.equal(toneColor(1, [0, 0, 0], [200, 100, 50]), 'rgb(200, 100, 50)');
  assert.equal(polygonLevel(-500, -400), 'lower');
  assert.equal(polygonLevel(-500, undefined), 'default');
});

test('tags: names stay readable and never overlap one another', () => {
  assert.equal(tagName('  NoDDyhh ', 12), 'NoDDyhh');
  assert.equal(tagName('averyveryverylongname', 10), 'averyvery…');
  const tags = [1, 2, 3, 4, 5].map((id) => ({ id, x: 100, y: 100, width: 60, height: 18 }));
  const placed = layoutTags({ tags, discs: [{ x: 100, y: 100 }], radius: 6, bounds: { x: 0, y: 0, width: 400, height: 400 } });
  assert.equal(placed.length, 5);
  for (const a of placed) {
    for (const b of placed) {
      if (a === b) continue;
      const apart = a.rect.x + a.rect.width <= b.rect.x || b.rect.x + b.rect.width <= a.rect.x || a.rect.y + a.rect.height <= b.rect.y || b.rect.y + b.rect.height <= a.rect.y;
      assert.ok(apart, `tags ${a.id} and ${b.id} overlap`);
    }
  }
  assert.equal(placed[0].displaced, false);
});

test('tags: a tag at the edge is kept inside the map', () => {
  const [placed] = layoutTags({ tags: [{ id: 1, x: 398, y: 2, width: 60, height: 18 }], discs: [], radius: 6, bounds: { x: 0, y: 0, width: 400, height: 400 } });
  assert.ok(placed.rect.x + placed.rect.width <= 400);
  assert.ok(placed.rect.y >= 0);
});

test('tags: a free preferred spot is kept from the last frame', () => {
  const [placed] = layoutTags({ tags: [{ id: 1, x: 100, y: 100, width: 40, height: 16 }], discs: [], radius: 6, bounds: { x: 0, y: 0, width: 400, height: 400 }, prefer: new Map([[1, 3]]) });
  assert.equal(placed.spot, 3);
});

test('map view: following zooms in on the player and only moves once they near the edge', () => {
  const start = followView({ view: base, base, point: { x: 0.5, y: 0.5 } });
  assert.equal(zoomLevel(start, base), FOLLOW_ZOOM);
  assert.ok(Math.abs(start.x + start.width / 2 - 0.5) < 1e-9);
  assert.equal(followView({ view: start, base, point: { x: 0.52, y: 0.5 } }), start);
  const moved = followView({ view: start, base, point: { x: 0.64, y: 0.5 } });
  assert.ok(Math.abs(moved.x + moved.width / 2 - 0.64) < 1e-9);
});
