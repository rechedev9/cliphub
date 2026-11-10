import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalEvent } from '../api/tactical.ts';
import { hitKind, hitLines, recentDamage } from './hit-lines.ts';

const secondsOf = (tick: number): number => tick / 64;
const flash = (tick: number, actor: number, blinded: { slot: number; seconds: number; enemy: boolean }[]): TacticalEvent => ({
  tick,
  kind: 'flash',
  actor_slot: actor,
  pos: [0, 0, 0],
  target_pos: [0, 0, 0],
  effect: { blinded },
});

test('hit lines: a hit shows for a second and fades, newest last', () => {
  const hits = [
    { tick: 640, actor_slot: 1, target_slot: 7, damage: 27, weapon: 'ak47', headshot: true },
    { tick: 672, actor_slot: 2, target_slot: 8, damage: 60, weapon: 'hegrenade' },
  ];
  const lines = hitLines({ hits, events: [], nowSeconds: 10.5, secondsOf });
  assert.deepEqual(
    lines.map((line) => [line.from, line.to, line.kind, line.amount, line.headshot, line.alpha]),
    [
      [1, 7, 'bullet', 27, true, 0.5],
      [2, 8, 'he', 60, false, 1],
    ],
  );
  assert.equal(hitLines({ hits, events: [], nowSeconds: 9.9, secondsOf }).length, 0);
  assert.equal(hitLines({ hits, events: [], nowSeconds: 11.6, secondsOf }).length, 0);
});

test('hit lines: a flash draws a line to each blinded enemy only', () => {
  const events = [flash(640, 3, [{ slot: 7, seconds: 2.4, enemy: true }, { slot: 4, seconds: 1, enemy: false }])];
  const lines = hitLines({ hits: [], events, nowSeconds: 10.25, secondsOf });
  assert.deepEqual(lines.map((line) => [line.from, line.to, line.kind, line.amount]), [[3, 7, 'flash', 2.4]]);
});

test('hit lines: fire weapons read as fire', () => {
  assert.equal(hitKind('inferno'), 'fire');
  assert.equal(hitKind('molotov'), 'fire');
  assert.equal(hitKind('awp'), 'bullet');
  assert.equal(hitKind(undefined), 'bullet');
});

test('recent damage: a hurt player shows it briefly, the latest hit strongest', () => {
  const hits = [
    { tick: 640, actor_slot: 1, target_slot: 7, damage: 27 },
    { tick: 656, actor_slot: 2, target_slot: 7, damage: 10 },
    { tick: 640, actor_slot: 3, target_slot: 8, damage: 0 },
  ];
  const hurt = recentDamage({ hits, nowSeconds: 10.25, secondsOf });
  assert.deepEqual([...hurt.keys()], [7]);
  assert.equal(hurt.get(7), 1);
  assert.equal(recentDamage({ hits, nowSeconds: 11, secondsOf }).size, 0);
  assert.equal(recentDamage({ hits, nowSeconds: 9.9, secondsOf }).size, 0);
});
