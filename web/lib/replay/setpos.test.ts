import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalEvent, TacticalFrame, TacticalSample } from '../api/tactical.ts';
import { roundTimeline } from '../tactical-timeline.ts';
import { grenadeSetpos, nearestSample, playerSetpos } from './setpos.ts';

const sample = (slot: number, x: number, extra: Partial<TacticalSample> = {}): TacticalSample => ({ slot, x, y: 20, z: -167.97, yaw: 291.89, pitch: 352.62, health: 100, flags: 1, ...extra });
const frames: TacticalFrame[] = [
  { tick: 1000, samples: [sample(4, -1000), sample(5, 300)] },
  { tick: 1008, samples: [sample(4, -1012.03)] },
];
// Freeze time ends at tick 960: tick 1008 is 0.75 s into the round at 64 Hz.
const timeline = roundTimeline({ tick_start: 0, tick_freeze_end: 960, tick_end: 10000, tick_official: 10000 }, 64);
const base = { frames, players: [{ slot: 4, name: 'mopoz' }], round: 7, timeline };

test('playerSetpos reads the nearest recorded sample with the feet z and setang angles', () => {
  assert.deepEqual(playerSetpos({ ...base, slot: 4, tick: 1005 }), {
    name: 'mopoz',
    at: 'R7 · 0:00,8',
    command: 'setpos -1012.03 20.00 -167.97;setang -7.38 -68.11 0',
  });
  assert.equal(playerSetpos({ ...base, slot: 4, tick: 1004 })?.command, 'setpos -1000.00 20.00 -167.97;setang -7.38 -68.11 0');
});

test('nearestSample never interpolates: a tie takes the earlier sample and the rest the closer one', () => {
  const at = (tick: number): number | undefined => nearestSample({ frames, slot: 4, tick })?.tick;
  assert.equal(at(1004), 1000);
  assert.equal(at(1005), 1008);
  assert.equal(at(900), 1000);
  assert.equal(at(5000), 1008);
  assert.equal(nearestSample({ frames, slot: 4, tick: 1005 })?.sample.x, -1012.03);
});

test('nearestSample skips frames that lack the player and gives null without one', () => {
  assert.equal(nearestSample({ frames, slot: 5, tick: 1007 })?.tick, 1000);
  assert.equal(playerSetpos({ ...base, slot: 5, tick: 1007 })?.command, 'setpos 300.00 20.00 -167.97;setang -7.38 -68.11 0');
  assert.equal(playerSetpos({ ...base, slot: 5, tick: 1007 })?.name, 'Jugador 5');
  assert.equal(playerSetpos({ ...base, slot: 9, tick: 1004 }), null);
  assert.equal(playerSetpos({ ...base, frames: [], slot: 4, tick: 1004 }), null);
});

test('playerSetpos refuses a sample with a number that is not finite', () => {
  const broken: TacticalFrame[] = [{ tick: 1000, samples: [sample(4, Number.NaN)] }];
  assert.equal(playerSetpos({ ...base, frames: broken, slot: 4, tick: 1000 }), null);
});

const thrown: TacticalEvent = {
  tick: 1200,
  kind: 'smoke',
  actor_slot: 4,
  pos: [10, 20, 30],
  target_pos: [0, 0, 0],
  throw: { tick: 1100, pos: [-383.62762451171875, -902.1229858398438, -415.96875], yaw: -41.036346435546875, pitch: -1.6500091552734375 },
};

test('grenadeSetpos copies the thrower at the throw tick, raw floats rounded only at the end', () => {
  assert.deepEqual(grenadeSetpos({ event: thrown, players: base.players, round: 7, timeline }), {
    name: 'mopoz',
    at: 'R7 · 0:02,2',
    command: 'setpos -383.63 -902.12 -415.97;setang -1.65 -41.04 0',
    notice: 'Setpos del lanzamiento de mopoz (Humo) copiado',
  });
});

test('grenadeSetpos brings a demo pitch of 270 to 360 and a yaw past 180 into setang ranges', () => {
  const event: TacticalEvent = { ...thrown, throw: { tick: 1100, pos: [1, 2, 3], yaw: 291.89, pitch: 352.62 } };
  assert.equal(grenadeSetpos({ event, players: base.players, round: 7, timeline })?.command, 'setpos 1.00 2.00 3.00;setang -7.38 -68.11 0');
});

test('grenadeSetpos is null without a recorded throw or a thrower', () => {
  assert.equal(grenadeSetpos({ event: { ...thrown, throw: undefined }, players: base.players, round: 7, timeline }), null);
  assert.equal(grenadeSetpos({ event: { ...thrown, actor_slot: undefined }, players: base.players, round: 7, timeline }), null);
});
