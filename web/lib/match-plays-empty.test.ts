import test from 'node:test';
import assert from 'node:assert/strict';
import {
  MATCH_PLAYS_ANALYZING_DESCRIPTION,
  MATCH_PLAYS_ANALYZING_TITLE,
  MATCH_PLAYS_EMPTY_DESCRIPTION,
  MATCH_PLAYS_EMPTY_TITLE,
  MATCH_PLAYS_ERROR_DESCRIPTION,
  MATCH_PLAYS_ERROR_TITLE,
  matchPlanReady,
} from './match-plays-empty.ts';

test('pending parse copy never reads as Sin jugadas destacables', () => {
  assert.match(MATCH_PLAYS_ANALYZING_TITLE, /Analizando/);
  assert.match(MATCH_PLAYS_ANALYZING_DESCRIPTION, /jugadas aparecerán/);
  assert.notEqual(MATCH_PLAYS_ANALYZING_TITLE, MATCH_PLAYS_EMPTY_TITLE);
  assert.equal(/Sin jugadas|destacables|Prueba con otra/i.test(MATCH_PLAYS_ANALYZING_TITLE), false);
  assert.equal(/Sin jugadas|destacables|Prueba con otra/i.test(MATCH_PLAYS_ANALYZING_DESCRIPTION), false);
});

test('genuine empty and error stay distinct from analyzing', () => {
  assert.equal(MATCH_PLAYS_EMPTY_TITLE, 'Sin jugadas destacables');
  assert.match(MATCH_PLAYS_EMPTY_DESCRIPTION, /no encontró/);
  assert.match(MATCH_PLAYS_ERROR_TITLE, /No se pudieron cargar/);
  assert.notEqual(MATCH_PLAYS_ERROR_TITLE, MATCH_PLAYS_EMPTY_TITLE);
  assert.notEqual(MATCH_PLAYS_ERROR_DESCRIPTION, MATCH_PLAYS_ANALYZING_DESCRIPTION);
  assert.equal(/Analizando/i.test(MATCH_PLAYS_EMPTY_TITLE), false);
  assert.equal(/Analizando/i.test(MATCH_PLAYS_ERROR_TITLE), false);
});

test('matchPlanReady treats unknown status as ready for fixtures', () => {
  const cases: { name: string; status: string | undefined; want: boolean }[] = [
    { name: 'undefined fixture', status: undefined, want: true },
    { name: 'parsed', status: 'parsed', want: true },
    { name: 'done', status: 'done', want: true },
    { name: 'scanned', status: 'scanned', want: false },
    { name: 'parsing', status: 'parsing', want: false },
  ];
  for (const { name, status, want } of cases) {
    assert.equal(matchPlanReady(status), want, name);
  }
});
