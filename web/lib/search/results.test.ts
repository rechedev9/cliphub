import { test } from 'node:test';
import assert from 'node:assert/strict';
import { searchResultsOf } from '../api/search.ts';
import { foldSearch, resultRow, sectionRows } from './results.ts';

test('foldSearch drops accents and reads & as y, like the server', () => {
  assert.equal(foldSearch('  Rotación & SEÑUELO '), 'rotacion y senuelo');
});

test('sectionRows finds the rail sections holding every word', () => {
  assert.deepEqual(sectionRows('libre').map((row) => row.href), ['/libreta']);
  assert.deepEqual(sectionRows('mis videos').map((row) => row.href), ['/mis-videos']);
  assert.deepEqual(sectionRows('   '), []);
});

test('resultRow links each kind and says what it is', () => {
  const lineup = resultRow({ kind: 'lineup', id: 'abc', title: 'Humo ventana', map: 'de_mirage', side: 'T', status: 'practice', detail: 'smoke' });
  assert.equal(lineup.href, '/libreta/lineups/abc');
  assert.equal(lineup.subtitle, 'Humo · Mirage · T · Para practicar');
  const call = resultRow({ kind: 'call', id: 'c1', title: 'Rotar', side: 'both', detail: 'a B\npor mid' });
  assert.equal(call.subtitle, 'a B · T y CT');
  assert.equal(resultRow({ kind: 'team', id: 't1', title: 'Vitality', detail: '3' }).subtitle, '3 partidas');
  assert.equal(resultRow({ kind: 'player', id: 'p1', title: 'ropz', detail: '1' }).subtitle, '1 partida');
  assert.equal(resultRow({ kind: 'match', id: 'm1', title: 'A vs B', detail: '2026-11-07', map: 'de_ancient' }).subtitle, '7 nov 2026 · Ancient');
});

test('searchResultsOf keeps only rows it can show', () => {
  const rows = searchResultsOf({ results: [{ kind: 'play', id: 'p', title: 'Exec' }, { kind: 'nope', id: 'x', title: 'y' }, { kind: 'call', id: 1 }] });
  assert.deepEqual(rows.map((row) => row.id), ['p']);
  assert.deepEqual(searchResultsOf(null), []);
});
