import test from 'node:test';
import assert from 'node:assert/strict';
import type { Leaderboard, LeaderboardRow } from '../api/baseline.ts';
import { versusRows } from './versus-board.ts';

const row = (steamid64: string, value: number, rounds = 50): LeaderboardRow => ({ rank: 0, steamid64, name: steamid64, rounds, matches: 1, value });
const board = (players: LeaderboardRow[], higher = true): Leaderboard => ({
  scope: 'library', map: '', source: 'library', metric: 'kpr', higher_is_better: higher, min_rounds: 20, matches: 1, rounds: 50, skipped: 0, players,
});

test('versus: the library and the pros rank together, pros marked', () => {
  const rows = versusRows(board([row('me', 0.8), row('mate', 0.5)]), board([row('pro1', 0.9), row('pro2', 0.6)]), 10);
  assert.deepEqual(rows.map((r) => [r.rank, r.steamid64, r.pro]), [[1, 'pro1', true], [2, 'me', false], [3, 'pro2', true], [4, 'mate', false]]);
});

test('versus: a pro already in the library is listed once, as a pro', () => {
  const rows = versusRows(board([row('pro1', 0.85, 80)]), board([row('pro1', 0.9)]), 10);
  assert.equal(rows.length, 1);
  assert.equal(rows[0]?.pro, true);
  assert.equal(rows[0]?.rounds, 80);
});

test('versus: lower first when lower is better, and the limit holds', () => {
  const rows = versusRows(board([row('me', 0.6)], false), board([row('pro1', 0.7), row('pro2', 0.5)], false), 2);
  assert.deepEqual(rows.map((r) => r.steamid64), ['pro2', 'me']);
});
