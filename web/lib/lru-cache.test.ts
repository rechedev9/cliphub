import test from 'node:test';
import assert from 'node:assert/strict';
import { LruCache } from './lru-cache.ts';

test('LRU promotes reads, evicts the least recent and caps a full match', () => {
  const cache = new LruCache<number, object>(4);
  const first = {};
  cache.set(1, first);
  for (const i of [2, 3, 4]) cache.set(i, {});
  assert.equal(cache.get(1), first);
  cache.set(5, {});
  assert.equal(cache.get(2), undefined);
  assert.equal(cache.get(1), first);
  for (let i = 6; i < 100; i++) {
    cache.set(i, {});
    assert.equal(cache.size, 4);
  }
  assert.equal(cache.get(1), undefined);
  cache.set(1, first);
  assert.equal(cache.get(1), first);
  cache.clear();
  assert.equal(cache.size, 0);
  assert.equal(cache.get(1), undefined);
});

test('replacement promotes without evicting an unrelated entry; invalid limits fail', () => {
  const cache = new LruCache<string, number>(2);
  cache.set('a', 1); cache.set('b', 2); cache.set('a', 3);
  assert.equal(cache.size, 2);
  cache.set('c', 4);
  assert.equal(cache.get('b'), undefined);
  assert.equal(cache.get('a'), 3);
  for (const limit of [0, -1, 1.5, Infinity, NaN]) assert.throws(() => new LruCache(limit), RangeError);
});
