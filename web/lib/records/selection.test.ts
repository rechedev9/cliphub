import test from 'node:test';
import assert from 'node:assert/strict';
import { countOf, selectedCount, type CountNoun } from './selection.ts';

const PARTIDA: CountNoun = { one: 'partida', many: 'partidas', feminine: true };
const LINEUP: CountNoun = { one: 'lineup', many: 'lineups', feminine: false };

test('the selection count agrees in gender and number with its noun', () => {
  assert.equal(selectedCount(1, PARTIDA), '1 partida seleccionada');
  assert.equal(selectedCount(14, PARTIDA), '14 partidas seleccionadas');
  assert.equal(selectedCount(1, LINEUP), '1 lineup seleccionado');
  assert.equal(selectedCount(2, LINEUP), '2 lineups seleccionados');
});

test('a bare count uses the singular only for one', () => {
  assert.equal(countOf(1, PARTIDA), '1 partida');
  assert.equal(countOf(0, LINEUP), '0 lineups');
});
