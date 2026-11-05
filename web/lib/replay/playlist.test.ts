import test from 'node:test';
import assert from 'node:assert/strict';
import { ReplayClock } from './clock.ts';
import {
  defaultMultiSelection,
  entryKey,
  indexOfKey,
  matchesToLoad,
  stepIndex,
  stripGroups,
  toggleMultiKey,
  withFocus,
} from './playlist.ts';

const A = '11111111-1111-4111-8111-111111111111';
const B = '22222222-2222-4222-8222-222222222222';
const entries = [
  { matchId: A, round: 1 },
  { matchId: A, round: 4, tick: 900 },
  { matchId: B, round: 2 },
  { matchId: B, round: 7 },
  { matchId: A, round: 9 },
];

test('playlist: steps stay inside the strip', () => {
  assert.equal(stepIndex(5, 0, -1), 0);
  assert.equal(stepIndex(5, 4, 1), 4);
  assert.equal(stepIndex(5, 2, 1), 3);
  assert.equal(stepIndex(0, 3, 1), 0);
});

test('playlist: loads the current match and the next one, once each', () => {
  assert.deepEqual(matchesToLoad(entries, 1, 1), [A, B]);
  assert.deepEqual(matchesToLoad(entries, 2, 1), [B]);
  assert.deepEqual(matchesToLoad(entries, 4, 1), [A]);
});

test('playlist: consecutive items of one match form one group', () => {
  assert.deepEqual(stripGroups(entries), [
    { matchId: A, start: 0, end: 2 },
    { matchId: B, start: 2, end: 4 },
    { matchId: A, start: 4, end: 5 },
  ]);
});

test('playlist: a filtered strip keeps the item that survived, else goes to the first', () => {
  assert.equal(entryKey(entries[1]), `${A}:4:900`);
  assert.equal(indexOfKey(entries.slice(1), entryKey(entries[3])), 2);
  assert.equal(indexOfKey(entries.slice(2), entryKey(entries[0])), 0);
  assert.equal(indexOfKey(entries, null), 0);
});

test('multi round: opens on the focus and its neighbours from the same match, capped', () => {
  assert.deepEqual(defaultMultiSelection(entries, 1), [`${A}:4:900`, `${A}:9:`, `${A}:1:`]);
  assert.deepEqual(defaultMultiSelection(entries, 1, 2), [`${A}:4:900`, `${A}:9:`]);
  assert.deepEqual(defaultMultiSelection([], 0), []);
});

test('multi round: toggling never drops the focus nor passes the cap', () => {
  assert.deepEqual(toggleMultiKey(['a', 'b'], 'a', 'a'), ['a', 'b']);
  assert.deepEqual(toggleMultiKey(['a', 'b'], 'b', 'a'), ['a']);
  assert.deepEqual(toggleMultiKey(['a', 'b'], 'c', 'a', 2), ['a', 'b']);
  assert.deepEqual(toggleMultiKey(['a'], 'c', 'a', 2), ['a', 'c']);
  assert.deepEqual(withFocus(['a', 'b'], 'c', 2), ['c', 'a']);
  assert.deepEqual(withFocus(['a', 'b'], 'b'), ['a', 'b']);
});

test('clock: listeners hear real moves until they leave', () => {
  const clock = new ReplayClock();
  const heard: number[] = [];
  const stop = clock.subscribe((seconds) => heard.push(seconds));
  clock.set(2);
  clock.set(2);
  clock.set(Number.NaN);
  clock.refresh();
  stop();
  clock.set(5);
  assert.deepEqual(heard, [2, 0, 0]);
  assert.equal(clock.seconds, 5);
});
