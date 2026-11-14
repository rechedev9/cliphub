import test from 'node:test';
import assert from 'node:assert/strict';
import { MatchAnalysisLoads } from './match-analysis-loads.ts';
import type { MatchAnalysis } from './match-analysis-loads.ts';

type Pending = { resolve: (value: string) => void; reject: (error: Error) => void };

function harness(seed?: { id: string; analysis: string }) {
  const calls: string[] = [];
  const pending = new Map<string, Pending>();
  const seen: ReadonlyMap<string, MatchAnalysis<string>>[] = [];
  const loads = new MatchAnalysisLoads<string>({
    load: (id) => {
      calls.push(id);
      return new Promise((resolve, reject) => pending.set(id, { resolve, reject }));
    },
    message: (error) => (error instanceof Error ? error.message : 'unknown'),
    onChange: (states) => seen.push(states),
    seed,
  });
  return { loads, calls, pending, seen };
}

const settle = () => new Promise((resolve) => setImmediate(resolve));

test('each wanted match loads once, however often it is wanted', () => {
  const { loads, calls } = harness();
  loads.want(['a', 'b']);
  loads.want(['b', 'a']);
  loads.prefetch('a');
  assert.deepEqual(calls, ['a', 'b']);
  assert.deepEqual(loads.states.get('a'), { state: 'loading' });
});

test('a prefetched match is ready by the time it is wanted', async () => {
  const { loads, calls, pending } = harness();
  loads.prefetch('b');
  pending.get('b')?.resolve('B');
  await settle();
  loads.want(['b']);
  assert.deepEqual(calls, ['b']);
  assert.deepEqual(loads.states.get('b'), { state: 'ready', analysis: 'B' });
});

test('every change hands out a new map so React sees it', async () => {
  const { loads, pending, seen } = harness();
  loads.want(['a']);
  pending.get('a')?.resolve('A');
  await settle();
  assert.equal(seen.length, 2);
  assert.notEqual(seen[0], seen[1]);
  assert.deepEqual(seen[0].get('a'), { state: 'loading' });
  assert.deepEqual(seen[1].get('a'), { state: 'ready', analysis: 'A' });
});

test('a failed match keeps its message until retried, and retry loads it again', async () => {
  const { loads, calls, pending } = harness();
  loads.want(['a']);
  pending.get('a')?.reject(new Error('sin posiciones'));
  await settle();
  loads.want(['a']);
  assert.deepEqual(loads.states.get('a'), { state: 'failed', message: 'sin posiciones' });
  loads.retry('a');
  assert.deepEqual(calls, ['a', 'a']);
  assert.deepEqual(loads.states.get('a'), { state: 'loading' });
});

test('the seed is ready without a request', () => {
  const { loads, calls } = harness({ id: 'a', analysis: 'A' });
  loads.want(['a']);
  assert.deepEqual(calls, []);
  assert.deepEqual(loads.states.get('a'), { state: 'ready', analysis: 'A' });
});

test('a new seed replaces the match, an equal one changes nothing', () => {
  const { loads, seen } = harness({ id: 'a', analysis: 'A' });
  loads.seed('a', 'A');
  assert.equal(seen.length, 0);
  loads.seed('a', 'A2');
  assert.deepEqual(loads.states.get('a'), { state: 'ready', analysis: 'A2' });
});
