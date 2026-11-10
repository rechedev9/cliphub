import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalFrame, TacticalSample } from '../api/tactical.ts';
import { roundTimeline } from '../tactical-timeline.ts';
import { playerSetpos } from './setpos.ts';

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

test('playerSetpos falls back to the earlier sample, and gives null without one', () => {
  assert.equal(playerSetpos({ ...base, slot: 5, tick: 1007 })?.command, 'setpos 300.00 20.00 -167.97;setang -7.38 -68.11 0');
  assert.equal(playerSetpos({ ...base, slot: 5, tick: 1007 })?.name, 'Jugador 5');
  assert.equal(playerSetpos({ ...base, slot: 9, tick: 1004 }), null);
  assert.equal(playerSetpos({ ...base, frames: [], slot: 4, tick: 1004 }), null);
});
