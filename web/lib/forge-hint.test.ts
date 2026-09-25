import test from 'node:test';
import assert from 'node:assert/strict';
import {
  FORGE_HINT_CHOOSE_PRESET,
  FORGE_HINT_DECIDE_MUSIC,
  FORGE_HINT_EMPTY_PLAYS,
  forgeHint,
} from './forge-hint.ts';

test('Shorts empty selection still asks for a jugada', () => {
  assert.equal(forgeHint(null, null), FORGE_HINT_EMPTY_PLAYS);
  assert.match(FORGE_HINT_EMPTY_PLAYS, /jugada/);
  assert.equal(/ronda/i.test(FORGE_HINT_EMPTY_PLAYS), false);
});

test('preset and music hints stay after a selection exists', () => {
  assert.equal(forgeHint('2 jugadas', null), FORGE_HINT_CHOOSE_PRESET);
  assert.equal(forgeHint('2 jugadas', 'Killfeed'), FORGE_HINT_DECIDE_MUSIC);
});
