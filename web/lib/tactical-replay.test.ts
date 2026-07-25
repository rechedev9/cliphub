// Unit tests for the presentation-only smoothing between position samples.
// The invariant these protect: interpolation moves a dot, and nothing else. The
// health, the flags and the slot always come from a real sample, and a player
// who died between two samples does not slide.
// Run: node --test "lib/**/*.test.ts"
import test from 'node:test';
import assert from 'node:assert/strict';
import { RADAR_LEVELS, TACTICAL_SAMPLE_FLAGS } from './api/tactical.ts';
import type { RadarCalibration, TacticalFrame, TacticalSample } from './api/tactical.ts';
import {
  dominantLevel,
  frameCursor,
  interpolatedSamples,
  isAlive,
  lerpAngleDegrees,
  sampleTrails,
} from './tactical-replay.ts';

const ALIVE = TACTICAL_SAMPLE_FLAGS.alive;

function sample(overrides: Partial<TacticalSample> & { slot: number }): TacticalSample {
  return {
    x: 0,
    y: 0,
    z: 0,
    yaw: 0,
    health: 100,
    flags: ALIVE,
    ...overrides,
  };
}

function frame(tick: number, samples: TacticalSample[]): TacticalFrame {
  return { tick, samples };
}

const FLAT: RadarCalibration = {
  map: 'de_mirage',
  source: 'overview',
  pos_x: -3230,
  pos_y: 1713,
  scale: 5,
  size: 1024,
};

const SPLIT: RadarCalibration = { ...FLAT, map: 'de_nuke', lower_altitude_max: -495 };

test('frameCursor: an empty round has no cursor', () => {
  assert.equal(frameCursor([], 10), undefined);
});

test('frameCursor: pins to the first and last frame outside the range', () => {
  const frames = [frame(100, []), frame(108, []), frame(116, [])];
  assert.deepEqual(frameCursor(frames, 50), { index: 0, alpha: 0 });
  assert.deepEqual(frameCursor(frames, 999), { index: 2, alpha: 0 });
});

test('frameCursor: finds the surrounding frames and the fraction between them', () => {
  const frames = [frame(100, []), frame(108, []), frame(116, [])];
  assert.deepEqual(frameCursor(frames, 100), { index: 0, alpha: 0 });
  assert.deepEqual(frameCursor(frames, 104), { index: 0, alpha: 0.5 });
  assert.deepEqual(frameCursor(frames, 108), { index: 1, alpha: 0 });
  assert.deepEqual(frameCursor(frames, 114), { index: 1, alpha: 0.75 });
});

test('frameCursor: binary search stays correct over a long round', () => {
  const frames = Array.from({ length: 400 }, (_, i) => frame(1000 + i * 8, []));
  const cursor = frameCursor(frames, 1000 + 137 * 8 + 4);
  assert.deepEqual(cursor, { index: 137, alpha: 0.5 });
});

test('lerpAngleDegrees: turns the short way across the 0/360 seam', () => {
  assert.equal(lerpAngleDegrees(350, 10, 0.5), 0);
  assert.equal(lerpAngleDegrees(10, 350, 0.5), 0);
  assert.equal(lerpAngleDegrees(0, 90, 0.5), 45);
  assert.equal(lerpAngleDegrees(20, 80, 0), 20);
  assert.equal(lerpAngleDegrees(20, 80, 1), 80);
});

test('interpolatedSamples: eases position and yaw, but never health or flags', () => {
  const frames = [
    frame(0, [sample({ slot: 1, x: 0, y: 0, yaw: 350, health: 80 })]),
    frame(8, [sample({ slot: 1, x: 100, y: 40, yaw: 10, health: 55 })]),
  ];
  const [interpolated] = interpolatedSamples(frames, { index: 0, alpha: 0.5 });
  assert.equal(interpolated.x, 50);
  assert.equal(interpolated.y, 20);
  assert.equal(interpolated.yaw, 0);
  assert.equal(interpolated.health, 80);
  assert.equal(interpolated.flags, ALIVE);
});

test('interpolatedSamples: an exact frame hit returns the real samples untouched', () => {
  const frames = [frame(0, [sample({ slot: 1, x: 7 })]), frame(8, [sample({ slot: 1, x: 99 })])];
  assert.equal(interpolatedSamples(frames, { index: 0, alpha: 0 }), frames[0].samples);
});

test('interpolatedSamples: a player who dies between samples does not slide', () => {
  const frames = [
    frame(0, [sample({ slot: 1, x: 0, flags: ALIVE })]),
    frame(8, [sample({ slot: 1, x: 100, flags: 0, health: 0 })]),
  ];
  const [held] = interpolatedSamples(frames, { index: 0, alpha: 0.5 });
  assert.equal(held.x, 0);
  assert.ok(isAlive(held));
});

test('interpolatedSamples: a slot missing from the next frame holds its last position', () => {
  const frames = [frame(0, [sample({ slot: 1, x: 10 })]), frame(8, [sample({ slot: 2, x: 90 })])];
  const [held] = interpolatedSamples(frames, { index: 0, alpha: 0.5 });
  assert.equal(held.x, 10);
});

test('interpolatedSamples: a cursor past the last frame has nothing to ease towards', () => {
  const frames = [frame(0, [sample({ slot: 1, x: 10 })])];
  assert.deepEqual(interpolatedSamples(frames, { index: 0, alpha: 0.5 }), frames[0].samples);
});

test('sampleTrails: keeps the window, oldest point first', () => {
  const frames = [
    frame(0, [sample({ slot: 1, x: 0 })]),
    frame(64, [sample({ slot: 1, x: 10 })]),
    frame(128, [sample({ slot: 1, x: 20 })]),
    frame(192, [sample({ slot: 1, x: 30 })]),
  ];
  const trails = sampleTrails(frames, { index: 3, alpha: 0 }, 64, 2);
  assert.deepEqual(trails.get(1)?.map((point) => point.x), [10, 20, 30]);
});

test('sampleTrails: a dead player leaves no trail', () => {
  const frames = [
    frame(0, [sample({ slot: 1, x: 0 })]),
    frame(64, [sample({ slot: 1, x: 10, flags: 0, health: 0 })]),
  ];
  const trails = sampleTrails(frames, { index: 1, alpha: 0 }, 64, 2);
  assert.deepEqual(trails.get(1)?.map((point) => point.x), [0]);
});

test('dominantLevel: a single-level map is always the default layer', () => {
  const below = [sample({ slot: 1, z: -900 })];
  assert.equal(dominantLevel(FLAT, below), RADAR_LEVELS.default);
});

test('dominantLevel: the layer most live players are on wins, ties keep default', () => {
  const lower = [sample({ slot: 1, z: -900 }), sample({ slot: 2, z: -900 }), sample({ slot: 3, z: 100 })];
  assert.equal(dominantLevel(SPLIT, lower), RADAR_LEVELS.lower);

  const tied = [sample({ slot: 1, z: -900 }), sample({ slot: 2, z: 100 })];
  assert.equal(dominantLevel(SPLIT, tied), RADAR_LEVELS.default);
});

test('dominantLevel: dead players do not vote', () => {
  const samples = [
    sample({ slot: 1, z: -900, flags: 0, health: 0 }),
    sample({ slot: 2, z: -900, flags: 0, health: 0 }),
    sample({ slot: 3, z: 100 }),
  ];
  assert.equal(dominantLevel(SPLIT, samples), RADAR_LEVELS.default);
});
