import test from 'node:test';
import assert from 'node:assert/strict';
import { TACTICAL_SAMPLE_FLAGS } from '../api/tactical.ts';
import type { TacticalEvent, TacticalFrame, TacticalRound, TacticalSide } from '../api/tactical.ts';
import { keyMoments } from './key-moments.ts';

type TacticalPlayerRound = TacticalRound['players'][number];

const player = (slot: number, side: TacticalSide): TacticalPlayerRound => ({
  slot,
  side,
  kills: 0,
  deaths: 0,
  assists: 0,
  damage: 0,
  equip_value: 0,
  money: 0,
  survived: true,
});
const kill = (tick: number, killer: number, victim: number, side: TacticalSide, opening = false): TacticalEvent => ({
  tick,
  kind: 'kill',
  actor_slot: killer,
  target_slot: victim,
  side,
  pos: [0, 0, 0],
  target_pos: [0, 0, 0],
  opening,
});

const players = [player(0, 'CT'), player(1, 'CT'), player(2, 'CT'), player(5, 'T'), player(6, 'T'), player(7, 'T')];
const names = new Map([
  [0, 'ana'],
  [1, 'bea'],
  [2, 'cris'],
  [5, 'dani'],
  [6, 'eva'],
  [7, 'fer'],
]);

test('key moments: opening, trade, plant, a defuse attempt and the clutch, in tick order', () => {
  const events: TacticalEvent[] = [
    kill(1000, 5, 0, 'T', true),
    kill(1100, 1, 5, 'CT'),
    { tick: 2000, kind: 'plant', actor_slot: 6, side: 'T', site: 'A', pos: [0, 0, 0], target_pos: [0, 0, 0] },
    kill(2500, 6, 1, 'T'),
    kill(3000, 2, 6, 'CT'),
  ];
  const defusing = TACTICAL_SAMPLE_FLAGS.alive | TACTICAL_SAMPLE_FLAGS.defusing;
  const frames: TacticalFrame[] = [2600, 2610, 2620, 2640, 2660].map((tick, i) => ({
    tick,
    samples: [{ slot: 2, x: 0, y: 0, z: 0, yaw: 0, health: 100, flags: i === 2 ? TACTICAL_SAMPLE_FLAGS.alive : defusing }],
  }));
  const moments = keyMoments({ round: { events, players }, frames, tickrate: 64, names });
  assert.deepEqual(
    moments.map((moment) => [moment.tick, moment.kind, moment.label]),
    [
      [1000, 'opening', 'dani mata a ana'],
      [1100, 'trade', 'bea mata a dani'],
      [2000, 'plant', 'eva en A'],
      [2500, 'clutch', 'cris, 1 contra 2'],
      [2600, 'defuse', 'cris'],
    ],
  );
});

test('key moments: a kill past the trade window is not a trade', () => {
  const events = [kill(1000, 5, 0, 'T', true), kill(1000 + 64 * 6, 1, 5, 'CT')];
  const moments = keyMoments({ round: { events, players }, frames: [], tickrate: 64, names });
  assert.deepEqual(moments.map((moment) => moment.kind), ['opening']);
});

test('key moments: kills after the round ended make no trade and no clutch', () => {
  // The round ends at 4000; the CTs then hunt the Ts who were saving.
  const events = [kill(1000, 5, 0, 'T', true), kill(3900, 6, 1, 'T'), kill(4050, 2, 6, 'CT'), kill(4100, 2, 7, 'CT')];
  const defusing = TACTICAL_SAMPLE_FLAGS.alive | TACTICAL_SAMPLE_FLAGS.defusing;
  const frames: TacticalFrame[] = [{ tick: 4200, samples: [{ slot: 2, x: 0, y: 0, z: 0, yaw: 0, health: 100, flags: defusing }] }];
  const moments = keyMoments({ round: { events, players }, frames, tickrate: 64, names, endTick: 4000 });
  // cris left alone at 3900 is a real clutch; the trade at 4050 and dani alone at 4100 are not.
  assert.deepEqual(moments.map((moment) => [moment.tick, moment.kind, moment.label]), [
    [1000, 'opening', 'dani mata a ana'],
    [3900, 'clutch', 'cris, 1 contra 3'],
  ]);
});
