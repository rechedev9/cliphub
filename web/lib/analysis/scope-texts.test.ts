import assert from 'node:assert/strict';
import { test } from 'node:test';
import { libraryTexts, toggledText } from './scope-texts.ts';
import { SCOPE_TEXTS_MAX } from './scope-replay-filters.ts';

test('libraryTexts keeps each tag and event once, whatever its case, and drops blanks', () => {
  const texts = libraryTexts({
    demos: [{ tags: ['Scrim', 'oficial'] }, { tags: ['scrim ', ''] }],
    series: [{ event: 'IEM Beijing 2026' }, { event: '' }, { event: 'iem beijing 2026' }],
  });
  assert.deepEqual(texts, { tags: ['oficial', 'Scrim'], events: ['IEM Beijing 2026'] });
});

test('toggledText adds and removes a value and stops at the server bound', () => {
  assert.deepEqual(toggledText(['a'], 'b'), ['a', 'b']);
  assert.deepEqual(toggledText(['a', 'b'], 'a'), ['b']);
  const full = Array.from({ length: SCOPE_TEXTS_MAX }, (_, i) => `t${i}`);
  assert.equal(toggledText(full, 'nuevo').length, SCOPE_TEXTS_MAX);
});
