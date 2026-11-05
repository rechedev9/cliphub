import test from 'node:test';
import assert from 'node:assert/strict';
import type { PlayerMatchLine } from '../api/metrics.ts';
import { trendPoints } from './player-trend.ts';

const line = (id: string, playedAt: string, overrides: Partial<PlayerMatchLine> = {}): PlayerMatchLine => ({
  match_id: id, map: 'de_mirage', played_at: playedAt, team_key: 'a', rounds: 20, kills: 16, deaths: 8, eco_kills: 0, wpa: 0.4,
  ...overrides,
});

const lines = [line('new', '2026-11-03T10:00:00Z'), line('old', '2026-11-01T10:00:00Z', { deaths: 0, kills: 5 })];

test('trend: points run oldest first, K/D without deaths counts the kills', () => {
  assert.deepEqual(trendPoints(lines, 'kd').map((p) => [p.matchId, p.value]), [['old', 5], ['new', 2]]);
  assert.deepEqual(trendPoints(lines, 'wpa').map((p) => p.value), [0.02, 0.02]);
});

test('trend: ADR needs the match damage, so a match without it drops out', () => {
  const damage = new Map([['new', { damage: 1700, rounds: 20 }]]);
  assert.deepEqual(trendPoints(lines, 'adr', damage).map((p) => [p.matchId, p.value]), [['new', 85]]);
});

test('trend: a match without rounds has no point', () => {
  assert.deepEqual(trendPoints([line('x', '2026-11-01T00:00:00Z', { rounds: 0 })], 'kpr'), []);
});
