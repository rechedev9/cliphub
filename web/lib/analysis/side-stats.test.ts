import assert from 'node:assert/strict';
import test from 'node:test';
import { SIDE_STAT_KEYS, type SideStat } from '../api/side-stats.ts';
import {
  SIDE_STAT_TILES,
  dimmedSide,
  formatSideValue,
  groupMeanText,
  parseCompareTeams,
  sideDelta,
  toggleCompareTeam,
  writeCompareTeams,
} from './side-stats.ts';

const A = '00000000-0000-4000-8000-00000000000a';
const B = '00000000-0000-4000-8000-00000000000b';

function stat(value: number | null, groupAvg: number | null): SideStat {
  return { value, count: 0, total: 0, group_avg: groupAvg, group_teams: groupAvg === null ? 0 : 3 };
}

test('the tiles follow the agreed order and cover every figure once', () => {
  assert.deepEqual(
    SIDE_STAT_TILES.map((tile) => tile.label),
    [
      'Mapas ganados', 'Rondas ganadas', 'Pistolas ganadas', 'Primera baja', '5 contra 4 ganadas', '4 contra 5 ganadas', 'Trades',
      'Daño de utilidad por ronda', 'Asistencias de flash por ronda', 'Rondas jugadas',
    ],
  );
  assert.deepEqual([...SIDE_STAT_TILES.map((tile) => tile.key)].sort(), [...SIDE_STAT_KEYS].sort());
});

test('a figure without data reads as a dash, never 0 %', () => {
  assert.equal(formatSideValue(null, 'pct'), '-');
  assert.equal(formatSideValue(0, 'pct'), '0 %');
  assert.equal(formatSideValue(8.456, 'per-round'), '8,46');
  assert.equal(formatSideValue(37, 'count'), '37');
});

test('a rate delta reads in points, a per round one in its unit, a count has none', () => {
  assert.deepEqual(sideDelta(stat(60, 75), 'pct'), { text: '▼ 15 pp', tone: 'worse' });
  assert.equal(sideDelta(stat(40, 30), 'count'), null);
  assert.deepEqual(sideDelta(stat(0.31, 0.2), 'per-round'), { text: '▲ 0,11', tone: 'better' });
  assert.deepEqual(sideDelta(stat(50.2, 50), 'pct'), { text: '=', tone: 'even' });
  assert.equal(sideDelta(stat(null, 50), 'pct'), null);
  assert.equal(sideDelta(stat(50, null), 'pct'), null);
});

test('the group mean reads under the tile', () => {
  assert.equal(groupMeanText(stat(60, 75), 'pct'), 'media 75 % · 3 equipos');
  assert.equal(groupMeanText(stat(60, null), 'pct'), 'Sin media del grupo');
});

test('the compare teams round trip through the URL, sorted and checked', () => {
  assert.deepEqual(parseCompareTeams(`${B},${A},${B}`), [A, B]);
  assert.deepEqual(parseCompareTeams(`${A},nope`), []);
  assert.deepEqual(parseCompareTeams(null), []);
  assert.equal(writeCompareTeams('tab=estadisticas', [B, A]), `tab=estadisticas&comparar=${encodeURIComponent(`${A},${B}`)}`);
  assert.equal(writeCompareTeams(`tab=estadisticas&comparar=${A}`, []), 'tab=estadisticas');
  assert.deepEqual(toggleCompareTeam([A], B), [A, B]);
  assert.deepEqual(toggleCompareTeam([A, B], A), [B]);
});

test('a chosen side greys the other one; both sides stay lit without a choice', () => {
  assert.equal(dimmedSide('T'), 'ct');
  assert.equal(dimmedSide('CT'), 't');
  assert.equal(dimmedSide(''), null);
});
