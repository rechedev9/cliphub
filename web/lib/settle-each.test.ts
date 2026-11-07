import test from 'node:test';
import assert from 'node:assert/strict';
import { SETTLE_EACH_LIMIT, settleEach, withoutKeys } from './settle-each.ts';

test('withoutKeys drops only the finished keys', () => {
  assert.deepEqual([...withoutKeys(new Set(['a', 'b', 'c']), ['a', 'x'])], ['b', 'c']);
});

test('settleEach keeps every outcome in item order and never exceeds the limit', async () => {
  let inFlight = 0;
  let peak = 0;
  const items = Array.from({ length: 10 }, (_, index) => index);
  const outcome = await settleEach(items, async (item) => {
    inFlight += 1;
    peak = Math.max(peak, inFlight);
    await new Promise((resolve) => setTimeout(resolve, 10 - item));
    inFlight -= 1;
    if (item % 3 === 0) throw new Error(`fail ${item}`);
    return item * 2;
  });
  assert.equal(peak, SETTLE_EACH_LIMIT);
  assert.deepEqual(outcome.done.map(({ item, value }) => [item, value]), [[1, 2], [2, 4], [4, 8], [5, 10], [7, 14], [8, 16]]);
  assert.deepEqual(outcome.failed.map(({ item }) => item), [0, 3, 6, 9]);
});

test('settleEach on no items does nothing', async () => {
  assert.deepEqual(await settleEach([], async () => 1), { done: [], failed: [] });
});
