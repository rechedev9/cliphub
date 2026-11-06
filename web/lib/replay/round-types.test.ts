import test from 'node:test';
import assert from 'node:assert/strict';
import type { TacticalRound } from '../api/tactical.ts';
import { roundTypeCounts, toggleValue } from './round-types.ts';

function round(number: number, t: string, ct: string, winner: 'CT' | 'T'): TacticalRound {
  return {
    number,
    winner,
    half: 1,
    players: [],
    economy: { ct_buy: 'full', t_buy: 'full' },
    class: { t_side: t, ct_side: ct, site: 'A', tags: [] },
  } as unknown as TacticalRound;
}

const rounds = [round(1, 'execute', 'hold', 'T'), round(2, 'execute', 'retake', 'CT'), round(3, 'fast', 'hold', 'CT')];

test('round types: counts ignore the pattern filters and keep selected empty types', () => {
  const counts = roundTypeCounts([], { t_patterns: ['execute'] }, rounds);
  assert.deepEqual(counts.t.map((entry) => [entry.pattern, entry.count, entry.selected]), [
    ['execute', 2, true],
    ['fast', 1, false],
  ]);
  assert.deepEqual(counts.ct.map((entry) => [entry.pattern, entry.count]), [
    ['hold', 1],
    ['retake', 1],
  ]);
});

test('round types: toggling a value adds it, removes it and clears the field when empty', () => {
  assert.deepEqual(toggleValue(undefined, 'a'), ['a']);
  assert.deepEqual(toggleValue(['a', 'b'], 'a'), ['b']);
  assert.equal(toggleValue(['a'], 'a'), undefined);
});
