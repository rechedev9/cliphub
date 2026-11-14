import test from 'node:test';
import assert from 'node:assert/strict';
import { RADAR_LEVELS } from '../api/tactical.ts';
import type { TrailPoint } from '../tactical-replay.ts';
import { TRAIL_STEPS, trailStrokes } from './trail-strokes.ts';

const line = (count: number, level?: TrailPoint['level']): TrailPoint[] => Array.from({ length: count }, (_, i) => ({ x: i * 10, y: 0, level }));
const exactAlpha = (i: number, count: number): number => 0.05 + 0.35 * (i / (count - 1));
const SINGLE = { level: RADAR_LEVELS.default, offFloorAlpha: 0.3 };

test('a long trail is a handful of strokes instead of one per segment', () => {
  const strokes = trailStrokes(line(31), SINGLE);
  assert.ok(strokes.length <= TRAIL_STEPS, `${strokes.length} strokes`);
  assert.equal(
    strokes.reduce((sum, stroke) => sum + stroke.segments.length, 0),
    30,
  );
});

test('each segment keeps the fade from the tail to the head within half a step', () => {
  const count = 31;
  const strokes = trailStrokes(line(count), SINGLE);
  for (const stroke of strokes) {
    for (const [from] of stroke.segments) {
      const i = from.x / 10 + 1;
      assert.ok(Math.abs(stroke.alpha - exactAlpha(i, count)) <= 0.35 / (2 * TRAIL_STEPS) + 1e-9, `segment ${i}: ${stroke.alpha}`);
    }
  }
  const alphas = strokes.map((stroke) => stroke.alpha);
  assert.deepEqual(alphas, [...alphas].sort((a, b) => a - b), 'older strokes are drawn first and fainter');
});

test('segments stay in order, so a stroke draws the same path the segments did', () => {
  const [first] = trailStrokes(line(31), SINGLE);
  const starts = first.segments.map(([from]) => from.x);
  assert.deepEqual(starts, [...starts].sort((a, b) => a - b));
  for (const [from, to] of first.segments) assert.equal(to.x - from.x, 10);
});

test('a stretch wholly on the other floor fades by the off-floor strength, or is dropped at 0', () => {
  const points: TrailPoint[] = [
    { x: 0, y: 0, level: RADAR_LEVELS.lower },
    { x: 10, y: 0, level: RADAR_LEVELS.lower },
    { x: 20, y: 0, level: RADAR_LEVELS.lower },
    { x: 30, y: 0, level: RADAR_LEVELS.default },
  ];
  const faded = trailStrokes(points, SINGLE);
  const offSegments = faded.filter((stroke) => stroke.segments.some(([from, to]) => from.level === RADAR_LEVELS.lower && to.level === RADAR_LEVELS.lower));
  assert.ok(offSegments.length > 0);
  for (const stroke of offSegments) assert.ok(stroke.alpha <= 0.4 * 0.3 + 1e-9, `${stroke.alpha}`);
  // The segment that reaches the shown floor keeps full strength.
  assert.ok(faded.some((stroke) => stroke.segments.some(([, to]) => to.level === RADAR_LEVELS.default) && stroke.alpha > 0.3));

  const dropped = trailStrokes(points, { ...SINGLE, offFloorAlpha: 0 });
  assert.equal(dropped.reduce((sum, stroke) => sum + stroke.segments.length, 0), 1);
});

test('a point without a floor stands on the floor the plate shows', () => {
  const strokes = trailStrokes(line(3), { level: RADAR_LEVELS.lower, offFloorAlpha: 0 });
  assert.equal(strokes.reduce((sum, stroke) => sum + stroke.segments.length, 0), 2);
});

test('fewer than two points draw nothing', () => {
  assert.deepEqual(trailStrokes([], SINGLE), []);
  assert.deepEqual(trailStrokes(line(1), SINGLE), []);
});
