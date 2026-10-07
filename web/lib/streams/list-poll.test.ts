import assert from 'node:assert/strict';
import test from 'node:test';
import { readIfCurrent } from './list-poll.ts';

function deferred<T>(): { promise: Promise<T>; resolve: (value: T) => void } {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((res) => {
    resolve = res;
  });
  return { promise, resolve };
}

test('a list read started before delete does not publish the deleted row', async () => {
  let generation = 0;
  const published: string[][] = [];
  const beforeDelete = deferred<string[]>();
  const afterDelete = deferred<string[]>();

  const started = generation;
  const stale = readIfCurrent(
    () => beforeDelete.promise,
    () => generation === started,
  ).then((rows) => {
    if (rows !== null) published.push(rows);
  });

  generation += 1;
  const restarted = generation;
  const fresh = readIfCurrent(
    () => afterDelete.promise,
    () => generation === restarted,
  ).then((rows) => {
    if (rows !== null) published.push(rows);
  });

  afterDelete.resolve([]);
  await fresh;
  beforeDelete.resolve(['deleted-stream', 'other-stream']);
  await stale;

  assert.deepEqual(published, [[]], 'the pre-delete list must not replace the list read after delete');
});

test('a read that is still current publishes its rows, including odd ids', async () => {
  const samples = [
    [],
    ['stream'],
    ['ñ'.repeat(200)],
    ['partida/../x'],
    ['id with spaces'],
  ];
  for (const sample of samples) {
    const rows = await readIfCurrent(async () => sample, () => true);
    assert.deepEqual(rows, sample);
  }
});

test('twenty overlapping pre-delete reads stay unpublished after one restart', async () => {
  let generation = 0;
  const published: string[][] = [];
  const gates = Array.from({ length: 20 }, () => deferred<string[]>());
  const pending = gates.map((gate, index) => {
    const started = generation;
    return readIfCurrent(
      () => gate.promise,
      () => generation === started,
    ).then((rows) => {
      if (rows !== null) published.push(rows);
      return index;
    });
  });

  generation += 1;
  const restarted = generation;
  const freshGate = deferred<string[]>();
  const fresh = readIfCurrent(
    () => freshGate.promise,
    () => generation === restarted,
  ).then((rows) => {
    if (rows !== null) published.push(rows);
  });

  freshGate.resolve([]);
  await fresh;
  for (const gate of gates) gate.resolve(['deleted-stream']);
  await Promise.all(pending);

  assert.deepEqual(published, [[]]);
});
