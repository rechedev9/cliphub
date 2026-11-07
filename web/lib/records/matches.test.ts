import test from 'node:test';
import assert from 'node:assert/strict';
import { matchPlayedAtValue, matchSourceLabel } from './matches.ts';

test('the library names each demo source and leaves an unknown one blank', () => {
  assert.equal(matchSourceLabel({ source: 'steam' }), 'Steam');
  assert.equal(matchSourceLabel({ source: 'upload' }), 'Importada');
  assert.equal(matchSourceLabel({}), '');
});

test('an unreadable played-at date has no sort value', () => {
  assert.equal(matchPlayedAtValue({ playedAt: '2026-10-01T10:00:00Z' }), Date.UTC(2026, 9, 1, 10));
  assert.equal(matchPlayedAtValue({ playedAt: 'nunca' }), null);
});
