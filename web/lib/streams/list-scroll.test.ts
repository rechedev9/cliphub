import test from 'node:test';
import assert from 'node:assert/strict';
import { nearestScrollTop } from './list-scroll.ts';

test('a row already in view leaves the list where it is', () => {
  assert.equal(nearestScrollTop({ itemTop: 120, itemBottom: 170, scrollTop: 100, viewHeight: 300 }), null);
});

test('a row above the view scrolls the list up to its top', () => {
  assert.equal(nearestScrollTop({ itemTop: 40, itemBottom: 90, scrollTop: 100, viewHeight: 300 }), 40);
});

test('a row below the view scrolls the list just enough to show its bottom', () => {
  assert.equal(nearestScrollTop({ itemTop: 450, itemBottom: 500, scrollTop: 100, viewHeight: 300 }), 200);
});

test('a row taller than the view lines up with its top', () => {
  assert.equal(nearestScrollTop({ itemTop: 450, itemBottom: 900, scrollTop: 100, viewHeight: 300 }), 450);
});

test('a list that does not scroll shows every row and never moves', () => {
  assert.equal(nearestScrollTop({ itemTop: 2400, itemBottom: 2450, scrollTop: 0, viewHeight: 3000 }), null);
});
