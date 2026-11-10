import test from 'node:test';
import assert from 'node:assert/strict';
import { FOLLOW_ZOOM, MAX_ZOOM, boxWindow, followView, panView, radarImageFits, zoomLevel, zoomViewAt } from './map-view.ts';
import { radarProjection } from '../tactical-spatial.ts';
import { NAV_RADAR_PALETTE, floorBounds, floorPolygons, heightTone, navRadarModel, placeTints, polygonLevel, toneColor, toneRange } from './nav-radar.ts';
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

test('map view: the plate window covers the whole box, beside the view too', () => {
  const wide = { width: 1200, height: 600 };
  const zoomed = panView({ box: wide, view: zoomViewAt({ box: wide, view: base, base, cursor: { x: 600, y: 300 }, factor: 1.57 }), base, dx: 200, dy: 0 });
  const projection = radarProjection(wide, zoomed);
  const window = boxWindow(wide, projection);
  // The window's corners land on the box's corners.
  assert.equal(projection.offsetX + window.x * projection.size, 0);
  assert.equal(projection.offsetY + window.y * projection.size, 0);
  assert.equal(window.width * projection.size, wide.width);
  assert.equal(window.height * projection.size, wide.height);
  // And it holds the whole view, so nothing on screen falls outside the plate (within the fit's rounding).
  const px = 1 / projection.size;
  assert.ok(window.x <= zoomed.x + px && window.x + window.width >= zoomed.x + zoomed.width - px);
  assert.ok(window.y <= zoomed.y + px && window.y + window.height >= zoomed.y + zoomed.height - px);
  assert.ok(window.width > zoomed.width);
});

test('map view: the official radar only sits under a calibration read from its overview', () => {
  assert.equal(radarImageFits({ source: 'overview' }), true);
  assert.equal(radarImageFits({ source: 'derived' }), false);
});

test('nav radar: one floor of a two-floor map is its own polygons and its own extent', () => {
  // An upper square near the origin and a lower one far to the east.
  const vertices = Float32Array.from([0, 0, 0, 10, 0, 0, 10, 10, 0, 100, 0, -700, 120, 0, -700, 120, 20, -700]);
  const model = navRadarModel({ vertices, triangles: new Uint32Array(), areas: [{ id: 1, polygon: [0, 1, 2] }, { id: 2, polygon: [3, 4, 5] }] });
  assert.ok(model !== null);
  const upper = floorPolygons(model, -495, 'default');
  const lower = floorPolygons(model, -495, 'lower');
  assert.equal(upper.length, 1);
  assert.equal(lower.length, 1);
  assert.deepEqual(floorBounds(lower), { minX: 100, minY: 0, maxX: 120, maxY: 20 });
  assert.deepEqual(floorBounds(upper), { minX: 0, minY: 0, maxX: 10, maxY: 10 });
  assert.equal(floorBounds([]), null);
  assert.equal(floorPolygons(model, undefined, 'lower').length, 2);
});

test('nav radar: the greys are flat, the same range on any floor', () => {
  const [low, high] = [NAV_RADAR_PALETTE.low, NAV_RADAR_PALETTE.high];
  assert.ok(high[0] - low[0] <= 24);
  const flat = [{ points: new Float64Array(), z: 5, cx: 0, cy: 0 }, { points: new Float64Array(), z: 5, cx: 0, cy: 0 }];
  assert.deepEqual(toneRange(flat), { minZ: 5, maxZ: 5 });
});

test('nav radar: bombsites and the CT spawn are tinted from the ground near them, on their own floor', () => {
  const square = (x: number, z: number): number[] => [x, 0, z, x + 10, 0, z, x + 10, 10, z];
  const vertices = Float32Array.from([...square(0, 0), ...square(30, 0), ...square(2000, 0), ...square(10, 300), ...square(0, -700)]);
  const areas = [0, 1, 2, 3, 4].map((id) => ({ id, polygon: [id * 3, id * 3 + 1, id * 3 + 2] }));
  const model = navRadarModel({ vertices, triangles: new Uint32Array(), areas });
  assert.ok(model !== null);
  const callouts = [
    { name: 'BombsiteA', level: 'default' as const, center: [5, 5] as [number, number], samples: 1 },
    { name: 'CTSpawn', level: 'lower' as const, center: [5, 5] as [number, number], samples: 1 },
    { name: 'Hell', level: 'default' as const, center: [5, 5] as [number, number], samples: 1 },
  ];
  const tints = placeTints(model, callouts, -495);
  const byZ = (z: number, x: number): string | undefined => [...tints].find(([polygon]) => polygon.z === z && polygon.cx > x && polygon.cx < x + 10)?.[1];
  assert.equal(byZ(0, 0), NAV_RADAR_PALETTE.site);
  assert.equal(byZ(0, 30), NAV_RADAR_PALETTE.site);
  // Too far, at another height (the roof), and on the other floor's tint.
  assert.equal(byZ(0, 2000), undefined);
  assert.equal(byZ(300, 10), undefined);
  assert.equal(byZ(-700, 0), NAV_RADAR_PALETTE.spawn);
  assert.equal(placeTints(model, callouts, -495), tints);
});
