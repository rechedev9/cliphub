import test from 'node:test';
import assert from 'node:assert/strict';
import { createSlotPool } from './slot-pool.ts';

test('requests up to the limit start at once and the rest wait their turn', () => {
  const pool = createSlotPool(2);
  const started: string[] = [];
  const release = ['a', 'b', 'c', 'd'].map((id) => pool.request(() => started.push(id)));
  assert.deepEqual(started, ['a', 'b']);
  release[0]?.();
  assert.deepEqual(started, ['a', 'b', 'c']);
  release[1]?.();
  assert.deepEqual(started, ['a', 'b', 'c', 'd']);
});

test('a request cancelled while waiting never starts and keeps no slot', () => {
  const pool = createSlotPool(1);
  const started: string[] = [];
  const releaseA = pool.request(() => started.push('a'));
  const cancelB = pool.request(() => started.push('b'));
  pool.request(() => started.push('c'));
  cancelB();
  releaseA();
  assert.deepEqual(started, ['a', 'c']);
});

test('releasing twice frees only one slot', () => {
  const pool = createSlotPool(1);
  const started: string[] = [];
  const releaseA = pool.request(() => started.push('a'));
  pool.request(() => started.push('b'));
  pool.request(() => started.push('c'));
  releaseA();
  releaseA();
  assert.deepEqual(started, ['a', 'b']);
});
