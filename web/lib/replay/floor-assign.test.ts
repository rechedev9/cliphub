import assert from 'node:assert/strict';
import { test } from 'node:test';
import type { RadarCalibration, TacticalFrame } from '../api/tactical.ts';
import { FLOOR_BAND, STILL_TICKS, arcLevels, assignFloor, createFloorTimeline } from './floor-assign.ts';

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

/** Frames of one slot at 8 Hz (8 ticks apart), the way the scan samples a 64-tick demo. */
function track(zs: readonly number[]): TacticalFrame[] {
  return zs.map((z, index) => frame(index * 8, { 0: z }));
}

function levelsOf(frames: readonly TacticalFrame[]): string[] {
  const timeline = createFloorTimeline(NUKE, frames);
  return frames.map((_, index) => timeline.levelAt(0, index, 'default'));
}

function floorChanges(levels: readonly string[]): number {
  return levels.filter((level, index) => index > 0 && level !== levels[index - 1]).length;
}

test('real Nuke ramp: a player peeking up and down the long ramp stays on one floor', () => {
  // Round 11, slot 0 of the 2026-11 Nuke demo, 15 s at the foot of the ramp: the raw cut flips six times.
  const zs = [-560, -560, -560, -544.25, -535.5, -526.5, -517.25, -508.75, -505.25, -505.5, -503.5, -494.5, -489.75, -489.75, -489.75, -489.5, -485.25, -474.5, -465.5, -466, -472.25, -479.25, -481, -478.25, -475.25, -477.75, -486.25, -494, -499.5, -504.5, -509.25, -514, -519.25, -524.75, -531, -542.5, -560, -560, -560, -560, -560, -560, -560, -560, -560, -560, -560, -560, -560, -560, -560, -562.75, -567, -565.75, -563.5, -563.5, -563.25, -560, -546.75, -520.5, -506.5, -505, -516.25, -537, -530.25, -523.75, -520.75, -514.75, -504.5, -491.75, -479.25, -474.25, -477.5, -482, -487, -491, -492.5, -491.75, -491.25, -485.25, -477, -475.75, -480, -486.5, -492.5, -498.75, -505.25, -512, -517, -518, -518, -517.75, -511, -497.5, -481.5, -475.5, -478.75, -484, -490.25, -496.5];
  const raw = zs.map((z) => (z <= -495 ? 'lower' : 'default'));
  assert.equal(floorChanges(raw), 6);
  const levels = levelsOf(track(zs));
  assert.equal(floorChanges(levels), 0);
  assert.ok(levels.every((level) => level === 'lower'));
});

test('real Nuke ramp: a player who stops a hair under the cut is on the lower floor from the first still sample', () => {
  // Round 9, slot 7: walks down the T ramp from -416 and stands at -496 for a minute. The band alone held him on the upper floor.
  const walk = [-419.5, -428.5, -437.75, -446.5, -455, -462, -469.75, -476, -482, -488, -493.75];
  const still = [-496.25, -496.25, -496.25, -496.25, -496.5, -496.5, -496.75, -496.75, -496.75, -496.5, -496, -496, -495.5, -495.5, -495.5, -495.5, -495.5, -495.5, -495.5, -495.5];
  const levels = levelsOf(track([...walk, ...still]));
  assert.deepEqual(levels.slice(0, walk.length), walk.map(() => 'default'));
  assert.deepEqual(levels.slice(walk.length), still.map(() => 'lower'));
});

test('standing still drops the band only after two seconds and only for a living player', () => {
  const up = [-560, -540, -520, -500, -480];
  const brief = levelsOf(track([...up, ...Array.from({ length: 10 }, () => -480), -470]));
  assert.ok(brief.every((level) => level === 'lower'));
  const longer = Array.from({ length: STILL_TICKS / 8 + 1 }, () => -480);
  const standing = [...up.slice(0, -1), ...longer];
  const settled = levelsOf(track(standing));
  assert.equal(settled[up.length - 2], 'lower');
  assert.deepEqual(settled.slice(up.length - 1), longer.map(() => 'default'));
  const dead = track(standing).map((entry, index) => (index < up.length - 1 ? entry : { ...entry, samples: entry.samples.map((sample) => ({ ...sample, flags: 0 })) }));
  assert.ok(levelsOf(dead).every((level) => level === 'lower'));
});
