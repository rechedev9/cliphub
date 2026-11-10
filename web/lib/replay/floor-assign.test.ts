import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { RadarCalibration, TacticalFrame } from '../api/tactical.ts';
import { FLOOR_BAND, arcLevels, assignFloor, createFloorTimeline } from './floor-assign.ts';

const NUKE: RadarCalibration = { map: 'de_nuke', source: 'overview', pos_x: -3453, pos_y: 2887, scale: 7, size: 1024, lower_altitude_max: -495 };
const MIRAGE: RadarCalibration = { map: 'de_mirage', source: 'overview', pos_x: -3230, pos_y: 1713, scale: 5, size: 1024 };

function frame(tick: number, zs: Record<number, number>): TacticalFrame {
  return { tick, samples: Object.entries(zs).map(([slot, z]) => ({ slot: Number(slot), x: 0, y: 0, z, yaw: 0, health: 100, flags: 1 })) };
}

test('without a previous floor the altitude cut decides', () => {
  assert.equal(assignFloor({ calibration: NUKE, z: -494 }), 'default');
  assert.equal(assignFloor({ calibration: NUKE, z: -495 }), 'lower');
  assert.equal(assignFloor({ calibration: MIRAGE, z: -5000 }), 'default');
});

test('a player stays on the floor they were on until clearly past the cut', () => {
  const inside = -495 + FLOOR_BAND / 2;
  assert.equal(assignFloor({ calibration: NUKE, z: inside, previous: 'lower' }), 'lower');
  assert.equal(assignFloor({ calibration: NUKE, z: inside, previous: 'default' }), 'default');
  assert.equal(assignFloor({ calibration: NUKE, z: -495 + FLOOR_BAND + 1, previous: 'lower' }), 'default');
  assert.equal(assignFloor({ calibration: NUKE, z: -495 - FLOOR_BAND, previous: 'default' }), 'lower');
});

test('a ramp descent changes floor once, not on every sample that crosses the cut', () => {
  // Down the ramp with a jump on the way, then back up a little.
  const zs = [-400, -470, -500, -480, -520, -500, -540, -520, -560, -540];
  const frames = zs.map((z, index) => frame(index, { 1: z }));
  const timeline = createFloorTimeline(NUKE, frames);
  const levels = frames.map((_, index) => timeline.levelAt(1, index, 'default'));
  const changes = levels.filter((level, index) => index > 0 && level !== levels[index - 1]).length;
  assert.equal(changes, 1);
  assert.equal(levels[0], 'default');
  assert.equal(levels.at(-1), 'lower');
});

test('a floor never depends on how the playhead got there, and each player has their own', () => {
  const frames = [frame(0, { 1: -400, 2: -700 }), frame(1, { 1: -520, 2: -480 }), frame(2, { 1: -600, 2: -400 })];
  const timeline = createFloorTimeline(NUKE, frames);
  assert.deepEqual([2, 0, 1, 2].map((index) => timeline.levelAt(2, index, 'default')), ['default', 'lower', 'lower', 'default']);
  assert.deepEqual([0, 1, 2].map((index) => timeline.levelAt(1, index, 'default')), ['default', 'default', 'lower']);
});

test('a slot the round never recorded, a frame before its first, and a one-floor map answer the fallback or the upper floor', () => {
  const frames = [frame(0, { 1: -400 }), frame(1, { 1: -400, 2: -800 })];
  const timeline = createFloorTimeline(NUKE, frames);
  assert.equal(timeline.levelAt(9, 0, 'lower'), 'lower');
  assert.equal(timeline.levelAt(2, 0, 'default'), 'default');
  assert.equal(timeline.levelAt(2, 1, 'default'), 'lower');
  assert.equal(createFloorTimeline(MIRAGE, [frame(0, { 1: -5000 })]).levelAt(1, 0, 'default'), 'default');
});

test('a throw arcs on the thrower\'s floor until the flight reaches the landing floor', () => {
  const path = [{ z: -400 }, { z: -300 }, { z: -420 }, { z: -520 }, { z: -640 }];
  assert.deepEqual(arcLevels(NUKE, path, { from: 'default', landing: 'lower' }), ['default', 'default', 'default', 'lower', 'lower']);
  assert.deepEqual(arcLevels(NUKE, path, { from: 'default', landing: 'default' }), ['default', 'default', 'default', 'default', 'default']);
  assert.deepEqual(arcLevels(NUKE, [{ z: -700 }, { z: -500 }, { z: -300 }], { from: 'lower', landing: 'default' }), ['lower', 'lower', 'default']);
});
