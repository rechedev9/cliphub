import test from 'node:test';
import assert from 'node:assert/strict';
import {
  FORGE_HINT_CHOOSE_PRESET,
  FORGE_HINT_EMPTY_PLAYS,
  FORGE_HINT_LOADING_PRESETS,
  forgeHint,
} from './forge-hint.ts';
import { PRODUCE_READY_HINT } from './produce/copy.ts';

const NO_STYLE = { label: null, loading: false };

test('Shorts empty selection still asks for a jugada', () => {
  assert.equal(forgeHint(null, NO_STYLE), FORGE_HINT_EMPTY_PLAYS);
  assert.match(FORGE_HINT_EMPTY_PLAYS, /jugada/);
  assert.equal(/ronda/i.test(FORGE_HINT_EMPTY_PLAYS), false);
});

test('a selection with a style needs no music decision', () => {
  assert.equal(forgeHint('2 jugadas', NO_STYLE), FORGE_HINT_CHOOSE_PRESET);
  assert.equal(forgeHint('2 jugadas', { label: 'Killfeed', loading: false }), PRODUCE_READY_HINT);
});

test('while the styles load the hint waits instead of asking for a style', () => {
  assert.equal(forgeHint('2 jugadas', { label: null, loading: true }), FORGE_HINT_LOADING_PRESETS);
});
