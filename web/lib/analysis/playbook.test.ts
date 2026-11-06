import assert from 'node:assert/strict';
import test from 'node:test';
import { scaleBars, tempoBuckets } from './playbook.ts';

test('tempo buckets keep empty gaps and skip rounds without a tempo', () => {
  const rows = [{ tempo_seconds: 2 }, { tempo_seconds: 12.5 }, { tempo_seconds: null }, { tempo_seconds: 14 }];
  const buckets = tempoBuckets(rows, 5);
  assert.deepEqual(buckets.map((bucket) => [bucket.fromSeconds, bucket.items.length]), [[0, 1], [5, 0], [10, 2]]);
  assert.deepEqual(tempoBuckets(rows, 0), []);
});

test('bars scale to the peak and a single round stays visible', () => {
  assert.deepEqual(scaleBars([0, 1, 50, 100]), [0, 4, 50, 100]);
  assert.deepEqual(scaleBars([0, 0]), [0, 0]);
});
