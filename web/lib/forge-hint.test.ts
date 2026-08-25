import test from 'node:test';
import assert from 'node:assert/strict';
import {
  FORGE_HINT_CHOOSE_PRESET,
  FORGE_HINT_DECIDE_MUSIC,
  FORGE_HINT_EMPTY_PLAYS,
  forgeHint,
} from './forge-hint.ts';
import { FULL_DEMO_FORGE_HINT_EMPTY, FULL_DEMO_FORGE_HINT_ERROR } from './full-demo.ts';

test('Shorts empty selection still asks for a jugada', () => {
  assert.equal(forgeHint(null, null), FORGE_HINT_EMPTY_PLAYS);
  assert.match(FORGE_HINT_EMPTY_PLAYS, /jugada/);
  assert.equal(/ronda/i.test(FORGE_HINT_EMPTY_PLAYS), false);
});

test('preset and music hints stay after a selection exists', () => {
  assert.equal(forgeHint('2 jugadas', null), FORGE_HINT_CHOOSE_PRESET);
  assert.equal(forgeHint('2 jugadas', 'Killfeed'), FORGE_HINT_DECIDE_MUSIC);
});

test('Full Demo empty and error hints talk about rondas, not Shorts jugadas', () => {
  assert.equal(forgeHint(null, null, FULL_DEMO_FORGE_HINT_EMPTY), FULL_DEMO_FORGE_HINT_EMPTY);
  assert.equal(forgeHint(null, 'POV nativo', FULL_DEMO_FORGE_HINT_ERROR), FULL_DEMO_FORGE_HINT_ERROR);
  assert.match(FULL_DEMO_FORGE_HINT_EMPTY, /rondas/);
  assert.match(FULL_DEMO_FORGE_HINT_ERROR, /rondas/);
  assert.equal(/jugada/i.test(FULL_DEMO_FORGE_HINT_EMPTY), false);
  assert.equal(/jugada/i.test(FULL_DEMO_FORGE_HINT_ERROR), false);
});
