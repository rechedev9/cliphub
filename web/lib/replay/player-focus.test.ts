import assert from 'node:assert/strict';
import { test } from 'node:test';
import { focusFromQuery, inFocus, pickPlayer, playerInitials, writeFocus } from './player-focus.ts';

test('a click focuses one player and a second click gives the team back', () => {
  assert.deepEqual(pickPlayer([], 3, false), [3]);
  assert.deepEqual(pickPlayer([3], 3, false), []);
  assert.deepEqual(pickPlayer([3, 5], 5, false), [5]);
});

test('Shift and click adds a player or takes them out', () => {
  assert.deepEqual(pickPlayer([5], 3, true), [3, 5]);
  assert.deepEqual(pickPlayer([3, 5], 3, true), [5]);
});

test('everyone is in focus while nobody is chosen', () => {
  assert.equal(inFocus([], 4), true);
  assert.equal(inFocus([2], 4), false);
});

test('the focus round-trips through the URL and drops unknown slots', () => {
  const query = writeFocus('tab=replay&subject=9', [5, 2]);
  assert.equal(query, 'tab=replay&subject=5&subject=2');
  assert.deepEqual(focusFromQuery(new URLSearchParams(`${query}&subject=12&subject=x`), [2, 5]), [2, 5]);
  assert.equal(writeFocus(query, []), 'tab=replay');
});

test('initials skip the decoration around a name', () => {
  assert.equal(playerInitials('-_Odysseus_-'), 'OD');
  assert.equal(playerInitials('g1father'), 'G1');
  assert.equal(playerInitials('***'), '?');
});
