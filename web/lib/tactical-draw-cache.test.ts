import test from 'node:test';
import assert from 'node:assert/strict';
import { TacticalDrawCache } from './tactical-draw-cache.ts';
import { worldToRendered } from './tactical-transform.ts';
import type { TacticalGeometry } from './api/tactical.ts';
import type { TimelineEvent } from './tactical-timeline.ts';

const geometry: TacticalGeometry = {
  map: 'de_mirage', source: 'occupancy', cell_size: 64, levels: [], callouts: [], sample_count: 1,
  bounds: { min_x: 0, max_x: 100, min_y: 0, max_y: 100 },
  calibration: { map: 'de_mirage', source: 'overview', pos_x: -3230, pos_y: 1713, scale: 5, size: 1024 },
};
const entry: TimelineEvent = {
  seconds: 1, fraction: 0.1,
  event: { kind: 'kill', tick: 64, pos: [10, 20, 0], target_pos: [30, 40, 0] },
};

test('event coordinates are reused, exact and invalidated on size/calibration changes', () => {
  const cache = new TacticalDrawCache();
  const first = cache.eventPoints(geometry, 600, entry);
  assert.equal(cache.eventPoints(geometry, 600, entry), first);
  assert.deepEqual(first.at, worldToRendered(geometry.calibration, 10, 20, 600));
  assert.deepEqual(first.target, worldToRendered(geometry.calibration, 30, 40, 600));
  assert.notEqual(cache.eventPoints(geometry, 900, entry), first);
  const next = { ...geometry, calibration: { ...geometry.calibration, scale: 7 } };
  assert.deepEqual(cache.eventPoints(next, 600, entry).at, worldToRendered(next.calibration, 10, 20, 600));
  const before = cache.eventPoints(next, 600, entry);
  cache.clear();
  assert.notEqual(cache.eventPoints(next, 600, entry), before);
});

test('label measurements are bounded, font-specific and cleared when fonts finish loading', () => {
  const cache = new TacticalDrawCache();
  let calls = 0;
  const measure = () => ++calls;
  assert.equal(cache.labelWidth('12px mono', 'ABC', measure), 1);
  assert.equal(cache.labelWidth('12px mono', 'ABC', measure), 1);
  assert.equal(cache.labelWidth('14px mono', 'ABC', measure), 2);
  cache.clear();
  assert.equal(cache.labelWidth('12px mono', 'ABC', measure), 3);
  for (let i = 0; i < 33; i++) cache.labelWidth('12px mono', String(i), measure);
  cache.labelWidth('12px mono', 'ABC', measure);
  assert.equal(calls, 37, 'an evicted label is measured again');
});
