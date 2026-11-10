import test from 'node:test';
import assert from 'node:assert/strict';
import type { LeaderboardRow } from '../api/baseline.ts';
import { roleChips } from './columns.ts';

const row: LeaderboardRow = {
  rank: 1,
  steamid64: '1',
  name: 'a',
  rounds: 30,
  matches: 2,
  value: 1,
  roles: { ct: 'a_anchor', t: 'b_lurk' },
  breakdown: {
    t: {
      role: 'b_lurk',
      roles: [{ key: 'b_lurk', pct: 62 }],
      map_role: 'canal',
      places: [{ key: 'canal', label: 'Canal', pct: 21 }],
    },
    ct: { roles: [], map_role: 'allround', places: [] },
  },
};

test('a chip per side reads the map position and its shares, T first', () => {
  assert.deepEqual(roleChips(row, ''), [
    { side: 't', label: 'Canal', breakdown: 'Lurk de B 62 % · Canal 21 %' },
    { side: 'ct', label: 'Todoterreno', breakdown: '' },
  ]);
});

test('the side filter keeps one chip, and a row without roles has none', () => {
  assert.deepEqual(
    roleChips(row, 'CT').map((chip) => chip.side),
    ['ct'],
  );
  assert.deepEqual(roleChips({ ...row, roles: undefined, breakdown: undefined }, ''), []);
});

test('without a breakdown the chip falls back to the side role', () => {
  assert.deepEqual(roleChips({ ...row, breakdown: undefined }, 'T'), [{ side: 't', label: 'Lurk de B', breakdown: '' }]);
});
