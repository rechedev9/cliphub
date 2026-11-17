import test from 'node:test';
import assert from 'node:assert/strict';
import { isSamePick, planPick } from './origin-pick.ts';

const A = '76561198000000001';
const B = '76561198000000002';
const C = '76561198000000003';

test('planPick reads the anchor and then each filmed teammate once', () => {
  const cases = [
    { name: 'single POV', plan: { target: { steamid64: A }, segments: [{ id: 's1', round: 1 }] }, want: [A] },
    {
      name: 'multi POV',
      plan: { target: { steamid64: A }, segments: [{ id: 's1', round: 1, pov: { steamid64: B } }, { id: 's2', round: 2, pov: { steamid64: A } }, { id: 's3', round: 3, pov: { steamid64: B } }] },
      want: [A, B],
    },
    { name: 'no target', plan: { segments: [] }, want: [] },
  ];
  for (const { name, plan, want } of cases) assert.deepEqual(planPick(plan), want, name);
});

test('isSamePick needs the same anchor and the same teammates', () => {
  const cases = [
    { name: 'same single player', a: [A], b: [A], want: true },
    { name: 'teammates in another order', a: [A, B, C], b: [A, C, B], want: true },
    { name: 'another anchor', a: [A, B], b: [B, A], want: false },
    { name: 'a teammate more', a: [A], b: [A, B], want: false },
    { name: 'another teammate', a: [A, B], b: [A, C], want: false },
    { name: 'empty pick', a: [], b: [], want: false },
  ];
  for (const { name, a, b, want } of cases) assert.equal(isSamePick(a, b), want, name);
});
