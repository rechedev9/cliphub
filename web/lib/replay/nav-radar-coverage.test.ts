import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_GROW, floorPolygons, navRadarModel } from './nav-radar.ts';
import type { NavPolygon } from './nav-radar.ts';
import { NAV_PATCHES } from './nav-patches.ts';

// Real alive samples of one Nuke demo; over all 193k of them the plate covers 99.97% (upper) and 99.92% (lower).
const CUT = -495;

function segmentDistance(options: { x: number; y: number; ax: number; ay: number; bx: number; by: number }): number {
  const { x, y, ax, ay, bx, by } = options;
  const dx = bx - ax;
  const dy = by - ay;
  const length = dx * dx + dy * dy;
  const t = length > 0 ? Math.min(1, Math.max(0, ((x - ax) * dx + (y - ay) * dy) / length)) : 0;
  return Math.hypot(x - ax - t * dx, y - ay - t * dy);
}

// Inside a polygon or within NAV_GROW of its edge: how far paintFloors grows each outline.
function isPainted(options: { polygons: readonly NavPolygon[]; x: number; y: number }): boolean {
  const { polygons, x, y } = options;
  return polygons.some(({ points }) => {
    let inside = false;
    for (let i = 0, j = points.length - 2; i < points.length; j = i, i += 2) {
      if (segmentDistance({ x, y, ax: points[j], ay: points[j + 1], bx: points[i], by: points[i + 1] }) <= NAV_GROW) return true;
      if (points[i + 1] > y !== points[j + 1] > y && x < ((points[j] - points[i]) * (y - points[i + 1])) / (points[j + 1] - points[i + 1]) + points[i]) inside = !inside;
    }
    return inside;
  });
}

function polygon(points: readonly number[], z: number): NavPolygon {
  return { points: Float64Array.from(points), z, lowZ: z, highZ: z, cx: points[0], cy: points[1] };
}

test('nav radar: a spot a few units off a wall is still floor', () => {
  // Round 1, upper floor: the mesh stops 6.25 units short of this player.
  const upperWall = [polygon([1641.8, -642.2, 1641.8, -1030, 1647, -1030, 1647, -724], -298)];
  assert.equal(isPainted({ polygons: upperWall, x: 1653.25, y: -868.75 }), true);
  // Round 1, lower floor: 4 units short.
  const lowerWall = [polygon([1012.5, -427, 985.5, -596.5, 1012.5, -596.5], -636)];
  assert.equal(isPainted({ polygons: lowerWall, x: 1016.5, y: -544.25 }), true);
  // Ground nobody stood on stays black.
  assert.equal(isPainted({ polygons: upperWall, x: 1653.25 + 3 * NAV_GROW, y: -868.75 }), false);
});

test('nav radar: the gap between two neighbouring areas is closed', () => {
  // Round 1, upper floor: 3 and 4 units from the two areas that meet at x 1149.
  const areas = [polygon([1344, -532, 1149, -425.5, 1149, -463], -300), polygon([1152, -340, 1143, -416.5, 1149, -425.5], -300)];
  assert.equal(isPainted({ polygons: areas, x: 1146, y: -428.25 }), true);
});

test('nav radar: the ramp foot the mesh lacks comes from the map patches', () => {
  const model = navRadarModel({ vertices: new Float32Array(), triangles: new Uint32Array(), areas: [] }, NAV_PATCHES.de_nuke);
  assert.ok(model !== null);
  const upper = floorPolygons(model, CUT, 'default');
  const lower = floorPolygons(model, CUT, 'lower');
  // Round 9: a player at the ramp's foot, then another on the stretch the floor rule still keeps on the upper pane.
  assert.equal(isPainted({ polygons: upper, x: 620.25, y: 333 }), true);
  assert.equal(isPainted({ polygons: upper, x: 605, y: 356.5 }), true);
  assert.equal(isPainted({ polygons: upper, x: 564.25, y: 322.75 }), true);
  // Round 5, upper floor, and round 7 on the lower floor.
  assert.equal(isPainted({ polygons: upper, x: 464, y: -1422.75 }), true);
  assert.equal(isPainted({ polygons: lower, x: 827, y: 624.75 }), true);
  // A patch belongs to its own pane only, and the middle of nowhere stays unpainted.
  assert.equal(isPainted({ polygons: lower, x: 620.25, y: 333 }), false);
  assert.equal(isPainted({ polygons: upper, x: 0, y: 0 }), false);
});

test('nav radar: every patch is a closed outline on one pane', () => {
  for (const patch of NAV_PATCHES.de_nuke) {
    assert.ok(patch.points.length >= 6 && patch.points.length % 2 === 0);
    assert.ok(patch.level === 'default' || patch.level === 'lower');
  }
});
