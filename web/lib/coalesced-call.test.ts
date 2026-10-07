import assert from 'node:assert/strict';
import test from 'node:test';
import { createCoalescedCall } from './coalesced-call.ts';

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void; reject: (error: unknown) => void } {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

test('a refresh requested during an in-flight poll publishes only the later snapshot', async () => {
  const first = deferred<string[]>();
  const second = deferred<string[]>();
  let calls = 0;
  const refresh = createCoalescedCall(() => {
    calls += 1;
    return calls === 1 ? first.promise : second.promise;
  });

  const poll = refresh();
  const deleted = refresh();
  first.resolve(['match-still-listed']);
  // The delete already committed before this second read, so the server list is empty.
  second.resolve([]);
  const [fromPoll, fromDelete] = await Promise.all([poll, deleted]);

  assert.equal(calls, 2, 'the delete must cause one fresh read, not be dropped');
  assert.deepEqual(fromPoll, []);
  assert.deepEqual(fromDelete, []);
});

test('parallel refreshes collapse to the original read plus one rerun', async () => {
  const gates: Array<ReturnType<typeof deferred<number>>> = [];
  const refresh = createCoalescedCall(() => {
    const gate = deferred<number>();
    gates.push(gate);
    return gate.promise.then(() => gates.length);
  });

  const first = refresh();
  await Promise.resolve();
  const rest = Array.from({ length: 25 }, () => refresh());
  assert.equal(gates.length, 1);
  gates[0]?.resolve(0);
  await Promise.resolve();
  await Promise.resolve();
  assert.equal(gates.length, 2, 'a burst during the first read is one rerun, not 25');
  gates[1]?.resolve(0);
  const values = await Promise.all([first, ...rest]);
  assert.deepEqual(values, Array.from({ length: 26 }, () => 2));
});

test('a failed read is retried when a newer refresh is already waiting', async () => {
  const first = deferred<string>();
  const second = deferred<string>();
  let calls = 0;
  const refresh = createCoalescedCall(() => {
    calls += 1;
    return calls === 1 ? first.promise : second.promise;
  });

  const poll = refresh();
  const retry = refresh();
  first.reject(new Error('blip'));
  second.resolve('empty-after-delete');
  assert.deepEqual(await Promise.all([poll, retry]), ['empty-after-delete', 'empty-after-delete']);
});

test('odd snapshots are returned unchanged once they are the latest read', async () => {
  const samples: unknown[] = [null, '', 'ñ'.repeat(200), { id: 'partida/../x' }, []];
  for (const sample of samples) {
    const refresh = createCoalescedCall(async () => sample);
    assert.equal(await refresh(), sample);
  }
});
