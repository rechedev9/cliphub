import assert from 'node:assert/strict';
import test from 'node:test';
import { roundOrdinals, roundTitle } from './round-ordinals.ts';

test('roundOrdinals numbers only rounds with several plays, in list order', () => {
  const plays = [
    { id: 'a', round: 12 },
    { id: 'b', round: 12 },
    { id: 'c', round: 13 },
    { id: 'd', round: 16 },
    { id: 'e', round: 16 },
    { id: 'f', round: 16 },
  ];
  const ordinals = roundOrdinals(plays);
  assert.deepEqual(ordinals.get('a'), { index: 1, total: 2 });
  assert.deepEqual(ordinals.get('b'), { index: 2, total: 2 });
  assert.equal(ordinals.has('c'), false);
  assert.deepEqual(ordinals.get('f'), { index: 3, total: 3 });
});

test('roundTitle appends the ordinal only when there is one', () => {
  assert.equal(roundTitle(16, { index: 2, total: 3 }), 'Ronda 16 · 2/3');
  assert.equal(roundTitle(4, undefined), 'Ronda 4');
});
