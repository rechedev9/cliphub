import test from 'node:test';
import assert from 'node:assert/strict';
import { COLLECTION_ITEMS_MAX, COLLECTION_NOTE_MAX } from '../api/collections.ts';
import type { CollectionItem } from '../api/collections.ts';
import { addMoment, clampNote, collectionRefs, momentCountLabel, moveItem, playableCount, removeItem, setItemNote } from './bookmarks.ts';

const A = '11111111-1111-4111-8111-111111111111';
const B = '22222222-2222-4222-8222-222222222222';
const item = (match: string, round: number, tick: number, note = ''): CollectionItem => ({ match_id: match, round, tick, note });

test('bookmarks: the same moment is kept once and a new note replaces the old', () => {
  const first = addMoment([], item(A, 3, 100, '  retake  '));
  assert.equal(first.outcome, 'added');
  assert.deepEqual(first.items, [item(A, 3, 100, 'retake')]);
  const again = addMoment(first.items, item(A, 3, 100, 'mejor nota'));
  assert.equal(again.outcome, 'updated');
  assert.deepEqual(again.items, [item(A, 3, 100, 'mejor nota')]);
  assert.deepEqual(addMoment(again.items, item(A, 3, 100)).items, again.items);
});

test('bookmarks: a full collection refuses a new moment', () => {
  const full = Array.from({ length: COLLECTION_ITEMS_MAX }, (_, i) => item(A, 1, i));
  const result = addMoment(full, item(B, 1, 1));
  assert.equal(result.outcome, 'full');
  assert.equal(result.items.length, COLLECTION_ITEMS_MAX);
});

test('bookmarks: reorder, delete and note edits keep the rest in place', () => {
  const items = [item(A, 1, 1), item(A, 2, 2), item(B, 3, 3)];
  assert.deepEqual(moveItem(items, 2, -1).map((i) => i.round), [1, 3, 2]);
  assert.deepEqual(moveItem(items, 0, -1).map((i) => i.round), [1, 2, 3]);
  assert.deepEqual(moveItem(items, 0, 1).map((i) => i.round), [2, 1, 3]);
  assert.deepEqual(removeItem(items, 1).map((i) => i.round), [1, 3]);
  assert.equal(setItemNote(items, 2, ' humo tardío ')[2].note, 'humo tardío');
  assert.equal([...clampNote('ñ'.repeat(COLLECTION_NOTE_MAX + 5))].length, COLLECTION_NOTE_MAX);
});

test('bookmarks: the playlist skips moments whose match is gone', () => {
  const items = [item(A, 1, 10), item(B, 2, 20), item(A, 3, 30)];
  assert.deepEqual(collectionRefs(items, [B]), [
    { matchId: A, round: 1, tick: 10 },
    { matchId: A, round: 3, tick: 30 },
  ]);
  assert.equal(playableCount(items, [A]), 1);
  assert.equal(momentCountLabel(1), '1 momento');
  assert.equal(momentCountLabel(4), '4 momentos');
});
