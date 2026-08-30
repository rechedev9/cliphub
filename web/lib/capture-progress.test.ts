import assert from 'node:assert/strict';
import test from 'node:test';
import {
  captureProgressDetail,
  captureProgressPercent,
  parseCaptureProgress,
} from './capture-progress.ts';

test('percent prefers the orchestrator number over done/total', () => {
  const cases: { progress: Parameters<typeof captureProgressPercent>[0]; want: number }[] = [
    { progress: { done: 3, total: 4, percent: 82 }, want: 82 },
    { progress: { done: 3, total: 4 }, want: 75 },
    { progress: { done: 0, total: 4, percent: 0 }, want: 0 },
    { progress: { done: 0, total: 4, percent: 8 }, want: 8 },
    { progress: { done: 4, total: 4, percent: 100 }, want: 100 },
    { progress: { done: 1, total: 3, percent: 120 }, want: 100 },
    { progress: { done: 1, total: 3, percent: -4 }, want: 0 },
  ];
  for (const tt of cases) {
    assert.equal(captureProgressPercent(tt.progress), tt.want, JSON.stringify(tt.progress));
  }
});

test('detail names the in-flight segment instead of only completed clips', () => {
  const cases: { progress: Parameters<typeof captureProgressDetail>[0]; want: string }[] = [
    { progress: { done: 0, total: 4, percent: 0 }, want: 'Preparando captura local' },
    { progress: { done: 0, total: 4, percent: 8 }, want: 'Capturando 1/4' },
    { progress: { done: 3, total: 4, percent: 82 }, want: 'Capturando 4/4' },
    { progress: { done: 4, total: 4, percent: 100 }, want: 'Validando captura local' },
  ];
  for (const tt of cases) {
    assert.equal(captureProgressDetail(tt.progress), tt.want, JSON.stringify(tt.progress));
  }
});

test('parseCaptureProgress keeps percent only when the payload has a number', () => {
  assert.equal(parseCaptureProgress(undefined), undefined);
  assert.equal(parseCaptureProgress({ done: 1, total: 0 }), undefined);
  assert.deepEqual(parseCaptureProgress({ done: 3, total: 4 }), { done: 3, total: 4 });
  assert.deepEqual(parseCaptureProgress({ done: 3, total: 4, percent: 82.4 }), {
    done: 3,
    total: 4,
    percent: 82,
  });
  assert.deepEqual(parseCaptureProgress({ done: 0, total: 0 }), { done: 0, total: 0 });
  assert.deepEqual(
    parseCaptureProgress({ done: 64000, total: 172772, percent: 37, unit: 'ticks', label: 'ticks', stage: 'parse' }),
    { done: 64000, total: 172772, percent: 37, unit: 'ticks', label: 'ticks', stage: 'parse' },
  );
});
