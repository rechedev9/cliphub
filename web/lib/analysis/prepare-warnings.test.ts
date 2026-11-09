import assert from 'node:assert/strict';
import { test } from 'node:test';
import { prepareWarnings } from './prepare-warnings.ts';

const NOW = Date.parse('2026-11-09T12:00:00Z');
const roster = [
  { nick: 'krabeni', first_played_at: '2026-06-01T00:00:00Z', last_played_at: '2026-11-01T00:00:00Z', matches: 12 },
  { nick: 'nuevo', first_played_at: '2026-10-20T00:00:00Z', last_played_at: '2026-11-01T00:00:00Z', matches: 4 },
  { nick: 'viejo', first_played_at: '2026-06-01T00:00:00Z', last_played_at: '2026-10-05T00:00:00Z', matches: 6 },
  { nick: 'suplente', first_played_at: '2026-10-25T00:00:00Z', last_played_at: '2026-10-25T00:00:00Z', matches: 1 },
];

test('a fresh sample with one lineup says nothing', () => {
  assert.deepEqual(prepareWarnings({ roster: roster.slice(0, 1), playedAt: ['2026-10-01T00:00:00Z', '2026-11-01T00:00:00Z'], now: NOW }), []);
});

test('an old sample is called out with its age', () => {
  const [warning] = prepareWarnings({ roster: [], playedAt: ['2026-07-01T00:00:00Z'], now: NOW });
  assert.match(warning ?? '', /hace 131 días/);
});

test('a lineup change inside the sample names who joined and who left, not the stand-in', () => {
  const warnings = prepareWarnings({ roster, playedAt: ['2026-10-01T00:00:00Z', '2026-11-01T00:00:00Z'], now: NOW });
  assert.equal(warnings.length, 1);
  assert.match(warnings[0] ?? '', /nuevo entró después de la primera partida; viejo no juega las últimas/);
  assert.doesNotMatch(warnings[0] ?? '', /suplente/);
});
