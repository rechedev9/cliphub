import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_GROW, floorPolygons, isPainted, navRadarModel } from './nav-radar.ts';
import type { NavPolygon } from './nav-radar.ts';
import { NAV_PATCHES } from './nav-patches.ts';

// Real alive samples of one analysed Nuke demo (round, x, y, z), with the nav polygons next to them.
// Measured over all 193k alive samples of its 23 rounds: on the plate 87.5% (upper floor) and 73.9%
// (lower floor) before the outlines grew and the patches were added, 99.97% and 99.92% after.
const CUT = -495;

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
