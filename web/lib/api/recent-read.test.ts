import test from 'node:test';
import assert from 'node:assert/strict';
import { RecentRead } from './recent-read.ts';

function counter(fail = false): { calls: () => number; read: () => Promise<number> } {
  let n = 0;
  return {
    calls: () => n,
    read: () => {
      n += 1;
      return fail ? Promise.reject(new Error('down')) : Promise.resolve(n);
    },
  };
}

test('reads that land together or within the keep window share one request', async () => {
  let clock = 0;
  const source = counter();
  const shared = new RecentRead(source.read, { keepMs: 1000, now: () => clock });
  assert.equal(await Promise.all([shared.get(), shared.get()]).then(([a, b]) => a === b), true);
  clock = 900;
  assert.equal(await shared.get(), 1);
  assert.equal(source.calls(), 1);
});

test('a read older than the keep window is fetched again', async () => {
  let clock = 0;
  const source = counter();
  const shared = new RecentRead(source.read, { keepMs: 1000, now: () => clock });
  await shared.get();
  clock = 1000;
  assert.equal(await shared.get(), 2);
});

test('drop forgets the last read, so a write is never followed by a stale list', async () => {
  const source = counter();
  const shared = new RecentRead(source.read, { keepMs: 1000, now: () => 0 });
  await shared.get();
  shared.drop();
  assert.equal(await shared.get(), 2);
});

test('a failed read is not reused', async () => {
  const source = counter(true);
  const shared = new RecentRead(source.read, { keepMs: 1000, now: () => 0 });
  await assert.rejects(shared.get());
  await assert.rejects(shared.get());
  assert.equal(source.calls(), 2);
});
