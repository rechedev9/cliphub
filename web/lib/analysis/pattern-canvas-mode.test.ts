import test from 'node:test';
import assert from 'node:assert/strict';
import { patternModeFromQuery, patternsHref, writePatternMode } from './pattern-canvas-mode.ts';

test('the panel opens from its own key, or from the Replay form a team link carries', () => {
  assert.equal(patternModeFromQuery(new URLSearchParams('')), 'granadas');
  assert.equal(patternModeFromQuery(new URLSearchParams('modo=jugadores')), 'jugadores');
  assert.equal(patternModeFromQuery(new URLSearchParams('forma=calor')), 'jugadores');
  assert.equal(patternModeFromQuery(new URLSearchParams('forma=patrones')), 'patron');
  assert.equal(patternModeFromQuery(new URLSearchParams('forma=recorridos')), 'granadas');
  assert.equal(patternModeFromQuery(new URLSearchParams('modo=patron&forma=granadas')), 'patron');
});

test('writing a panel keeps the rest of the query and drops the Replay form', () => {
  assert.equal(writePatternMode('map=de_mirage&forma=calor', 'patron'), 'map=de_mirage&modo=patron');
  assert.equal(writePatternMode('map=de_mirage&modo=patron', 'granadas'), 'map=de_mirage');
});

test('the Buscador lives at /patrones, with the team after it', () => {
  assert.equal(patternsHref(''), '/patrones');
  assert.equal(patternsHref('a b'), '/patrones/a%20b');
});
