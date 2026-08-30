import assert from 'node:assert/strict';
import test from 'node:test';
import { jobProgressCount, jobProgressDisplay, jobProgressPercent, parseJobProgress } from './job-progress.ts';

test('jobProgressPercent prefers the worker percent and clamps', () => {
  const cases: Array<{ progress: Parameters<typeof jobProgressPercent>[0]; want: number }> = [
    { progress: { done: 64000, total: 172772, percent: 37 }, want: 37 },
    { progress: { done: 1, total: 4 }, want: 25 },
    { progress: { done: 0, total: 0, percent: 0 }, want: 0 },
    { progress: { done: 1, total: 2, percent: 120 }, want: 100 },
    { progress: { done: 1, total: 2, percent: -4 }, want: 0 },
  ];
  for (const tc of cases) {
    assert.equal(jobProgressPercent(tc.progress), tc.want, JSON.stringify(tc.progress));
  }
});

test('jobProgressCount prints current/total and the Spanish unit word', () => {
  assert.equal(jobProgressCount({ done: 64000, total: 172772, label: 'ticks' }), '64000 / 172772 ticks');
  assert.equal(jobProgressCount({ done: 8, total: 20, label: 'rondas' }), '8 / 20 rondas');
  assert.equal(jobProgressCount({ done: 2, total: 5, unit: 'clips' }), '2 / 5 clips');
  assert.equal(jobProgressCount({ done: 0, total: 3 }), '0 / 3');
});

test('a remounted snapshot resumes the same percent and count', () => {
  const stored = parseJobProgress({
    done: 64000,
    total: 172772,
    percent: 37,
    unit: 'ticks',
    label: 'ticks',
    stage: 'parse',
  });
  assert.deepEqual(stored, {
    done: 64000,
    total: 172772,
    percent: 37,
    unit: 'ticks',
    label: 'ticks',
    stage: 'parse',
  });
  if (stored === undefined) {
    assert.fail('expected a resume snapshot');
  }
  assert.equal(jobProgressPercent(stored), 37);
  assert.equal(jobProgressCount(stored), '64000 / 172772 ticks');
});

test('jobProgressDisplay omits percent and count when no snapshot exists', () => {
  assert.deepEqual(jobProgressDisplay(undefined), {});
  assert.deepEqual(jobProgressDisplay({ done: 8, total: 20, percent: 40, label: 'rondas' }), {
    percent: '40%',
    count: '8 / 20 rondas',
  });
  assert.deepEqual(jobProgressDisplay({ done: 3, total: 20, percent: 15, unit: 'clips', label: 'clips', stage: 'render' }), {
    percent: '15%',
    count: '3 / 20 clips',
  });
});

test('jobProgressDisplay omits encode-start 0/N compose and render placeholders', () => {
  assert.deepEqual(jobProgressDisplay({ done: 0, total: 20, percent: 0, unit: 'clips', label: 'clips', stage: 'render' }), {});
  assert.deepEqual(jobProgressDisplay({ done: 0, total: 8, percent: 0, unit: 'clips', label: 'clips', stage: 'compose' }), {});
  assert.deepEqual(jobProgressDisplay({ done: 0, total: 172772, percent: 0, unit: 'ticks', label: 'ticks', stage: 'parse' }), {
    percent: '0%',
    count: '0 / 172772 ticks',
  });
});

test('parseJobProgress keeps unit/label/stage and allows a 0/0 first write', () => {
  assert.equal(parseJobProgress(undefined), undefined);
  assert.equal(parseJobProgress({ done: 1, total: 0 }), undefined);
  assert.deepEqual(parseJobProgress({ done: 0, total: 0 }), { done: 0, total: 0 });
  assert.deepEqual(
    parseJobProgress({ done: 64000, total: 172772, percent: 37.2, unit: 'ticks', label: 'ticks', stage: 'parse' }),
    { done: 64000, total: 172772, percent: 37, unit: 'ticks', label: 'ticks', stage: 'parse' },
  );
});
