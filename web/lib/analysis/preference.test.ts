import test from 'node:test';
import assert from 'node:assert/strict';
import { preferenceExplanation, preferenceScoreText, preferenceWord } from './preference.ts';

test('preference: the score keeps its sign and reads as a lean only past the threshold', () => {
  assert.deepEqual([42, -15, 0].map(preferenceScoreText), ['+42', '-15', '0']);
  assert.deepEqual([42, 10, -30].map(preferenceWord), ['La busca', 'Sin preferencia clara', 'La evita']);
});

test('preference: the tooltip names each part with its sample', () => {
  const text = preferenceExplanation({ score: 31, veto: 0.5, play: 0.2, win: -0.1, vetoes: 4, matches: 6 });
  assert.match(text, /veto \(\+50, 4 picks y bans\)/);
  assert.match(text, /\(\+20\)/);
  assert.match(text, /\(-10, 6 partidas\)/);
  assert.doesNotMatch(text, /[–—]/);
});
