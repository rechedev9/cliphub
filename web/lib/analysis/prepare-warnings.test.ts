import assert from 'node:assert/strict';
import { test } from 'node:test';
import { prepareWarnings } from './prepare-warnings.ts';

const NOW = Date.parse('2026-11-09T12:00:00Z');
const roster = [
  { nick: 'krabeni', first_played_at: '2026-06-01T00:00:00Z', last_played_at: '2026-11-01T00:00:00Z', matches: 12 },
  { nick: 'nuevo', first_played_at: '2026-10-20T00:00:00Z', last_played_at: '2026-11-01T00:00:00Z', matches: 4 },
  { nick: 'viejo', first_played_at: '2026-06-01T00:00:00Z', last_played_at: '2026-10-05T00:00:00Z', matches: 6 },
  { nick: 'suplente', first_played_at: '2026-10-25T00:00:00Z', last_played_at: '2026-10-25T00:00:00Z', matches: 1 },
  { nick: 'antiguo', first_played_at: '2026-01-01T00:00:00Z', last_played_at: '2026-05-01T00:00:00Z', matches: 20 },
];
const PERIOD = ['2026-10-01T00:00:00Z', '2026-11-01T00:00:00Z'];

test('a fresh sample with one lineup says nothing', () => {
  assert.deepEqual(prepareWarnings({ roster: roster.slice(0, 1), periodPlayedAt: PERIOD, keptPlayedAt: PERIOD, now: NOW }), []);
});

test('the age comes from the matches the sample kept, not from the period', () => {
  const [warning] = prepareWarnings({ roster: [], periodPlayedAt: PERIOD, keptPlayedAt: ['2026-07-01T00:00:00Z'], now: NOW });
  assert.match(warning ?? '', /hace 131 días/);
  assert.deepEqual(prepareWarnings({ roster: [], periodPlayedAt: ['2026-07-01T00:00:00Z'], keptPlayedAt: null, now: NOW }), []);
});

test('a lineup change in the period names who joined and who left, not the stand-in or who left before it', () => {
  const warnings = prepareWarnings({ roster, periodPlayedAt: PERIOD, keptPlayedAt: ['2026-11-01T00:00:00Z'], now: NOW });
  assert.equal(warnings.length, 1);
  assert.match(warnings[0] ?? '', /La alineación cambió en este periodo: nuevo entró después de la primera partida; viejo no juega las últimas/);
  assert.doesNotMatch(warnings[0] ?? '', /suplente|antiguo/);
});
