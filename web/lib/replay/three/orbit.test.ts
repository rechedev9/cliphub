import test from 'node:test';
import assert from 'node:assert/strict';
import {
  ORBIT_MAX_ELEVATION,
  ORBIT_MIN_ELEVATION,
  framingOrbit,
  framingTop,
  orbitDrag,
  orbitPan,
  orbitPosition,
  orbitZoom,
  topPan,
  topZoom,
  unitsPerPixelAt,
} from './orbit.ts';
import type { OrbitState } from './orbit.ts';

const close = (actual: number, expected: number, epsilon = 1e-6): void => {
  assert.ok(Math.abs(actual - expected) <= epsilon, `${actual} is not ${expected}`);
};

const box = { min: { x: -2000, y: -1000, z: -100 }, max: { x: 2000, y: 1000, z: 300 } };

test('orbit: the camera sits distance away from the target at the given angles', () => {
  const state: OrbitState = { target: { x: 10, y: 20, z: 0 }, distance: 100, azimuth: 0, elevation: Math.PI / 2 - 1e-9 };
  const top = orbitPosition(state);
  close(top.x, 10, 1e-4);
  close(top.z, 100, 1e-4);
  const side = orbitPosition({ ...state, azimuth: Math.PI / 2, elevation: 0 });
  close(side.x, 10, 1e-9);
  close(side.y, 120, 1e-9);
  close(side.z, 0, 1e-9);
});

test('orbit: tilting stops short of the ground and of straight down', () => {
  const state: OrbitState = { target: { x: 0, y: 0, z: 0 }, distance: 1000, azimuth: 0, elevation: 0.5 };
  assert.equal(orbitDrag(state, 0, 10000).elevation, ORBIT_MAX_ELEVATION);
  assert.equal(orbitDrag(state, 0, -10000).elevation, ORBIT_MIN_ELEVATION);
  assert.ok(orbitDrag(state, 50, 0).azimuth < state.azimuth);
});

test('orbit: panning right moves the target left of the view, panning down moves it forward', () => {
  // Camera on +X looking at the origin: screen right is +Y, forward is -X.
  const state: OrbitState = { target: { x: 0, y: 0, z: 0 }, distance: 1000, azimuth: 0, elevation: 0.5 };
  const right = orbitPan(state, 10, 0, 2);
  close(right.target.x, 0);
  close(right.target.y, -20);
  const down = orbitPan(state, 0, 10, 2);
  close(down.target.x, -20);
  close(down.target.y, 0);
});

test('orbit: the wheel zooms within limits and away on a positive delta', () => {
  const state: OrbitState = { target: { x: 0, y: 0, z: 0 }, distance: 1000, azimuth: 0, elevation: 0.5 };
  assert.ok(orbitZoom(state, 100).distance > 1000);
  assert.ok(orbitZoom(state, -100).distance < 1000);
  let far = state;
  for (let i = 0; i < 200; i += 1) far = orbitZoom(far, 400);
  assert.equal(far.distance, 40000);
});

test('orbit: framing fits the whole box in a narrow view too', () => {
  const wide = framingOrbit(box, 50, 16 / 9);
  const narrow = framingOrbit(box, 50, 0.6);
  close(wide.target.x, 0);
  close(wide.target.y, 0);
  assert.ok(narrow.distance > wide.distance);
  const radius = 0.5 * Math.hypot(4000, 2000, 400);
  assert.ok(wide.distance * Math.sin((25 * Math.PI) / 180) >= radius * 0.9);
});

test('orbit: units per pixel grow with distance', () => {
  close(unitsPerPixelAt(1000, 90, 500), 4);
  assert.ok(unitsPerPixelAt(2000, 60, 500) > unitsPerPixelAt(1000, 60, 500));
});

test('top view: frames the box, pans with the pointer and zooms', () => {
  const top = framingTop(box, 1);
  close(top.x, 0);
  assert.ok(top.halfHeight >= 2000);
  const moved = topPan(top, 10, 10, 100);
  assert.ok(moved.x < top.x);
  assert.ok(moved.y > top.y);
  assert.ok(topZoom(top, -100).halfHeight < top.halfHeight);
});
