import test from 'node:test';
import assert from 'node:assert/strict';
import { SERVICE_UNAVAILABLE_CODE } from './api/types.ts';
import { DELETE_OFFLINE_MESSAGE } from './delete-error.ts';
import { formatBytes, freeableJobs, storageLoadErrorMessage, STORAGE_LOAD_ERROR_MESSAGE } from './storage.ts';

const GIB = 1024 ** 3;
const A = '11111111-1111-4111-8111-111111111111';
const B = '22222222-2222-4222-8222-222222222222';
const C = '33333333-3333-4333-8333-333333333333';
const D = '44444444-4444-4444-8444-444444444444';

test('formatBytes uses es-ES decimals and 1024-based units', () => {
  assert.equal(formatBytes(31.2 * GIB), '31,2 GB');
  assert.equal(formatBytes(18 * GIB), '18 GB');
  assert.equal(formatBytes(512), '512 B');
  assert.equal(formatBytes(1536), '1,5 KB');
  assert.equal(formatBytes(2.5 * 1024 ** 4), '2,5 TB');
});

test('formatBytes reads zero, negative and non-finite input as 0 B', () => {
  for (const value of [0, -5, Number.NaN, Number.POSITIVE_INFINITY]) assert.equal(formatBytes(value), '0 B');
});

test('formatBytes carries a value that rounds to 1024 into the next unit', () => {
  assert.equal(formatBytes(1024 * 1024 * 1024 - 1), '1 GB');
});

test('freeableJobs keeps jobs whose captures a render covers, biggest first, named after their match', () => {
  const usage = {
    totalBytes: 0,
    jobs: [
      { jobId: A, captureBytes: 2 * GIB, renderBytes: GIB, freeable: true },
      { jobId: B, captureBytes: 0, renderBytes: GIB, freeable: false },
      { jobId: C, captureBytes: 9 * GIB, renderBytes: 0, freeable: true },
      { jobId: D, captureBytes: 12 * GIB, renderBytes: 0, freeable: false },
    ],
  };
  const matches = [
    { id: A, map: 'de_inferno', player: 'ropz' },
    { id: B, map: 'Mirage' },
  ];
  assert.deepEqual(freeableJobs(usage, matches), [
    { jobId: C, label: 'Vídeo 33333333', captureBytes: 9 * GIB, renderBytes: 0 },
    { jobId: A, label: 'Inferno · ropz', captureBytes: 2 * GIB, renderBytes: GIB },
  ]);
});

test('freeableJobs names a match without a player after its map alone', () => {
  const usage = { totalBytes: 1, jobs: [{ jobId: B, captureBytes: 1, renderBytes: 0, freeable: true }] };
  assert.equal(freeableJobs(usage, [{ id: B, map: 'Mirage' }])[0].label, 'Mirage');
});

test('storageLoadErrorMessage tells an offline orchestrator apart from other failures', () => {
  const offline = Object.assign(new Error('analysis service unavailable'), { code: SERVICE_UNAVAILABLE_CODE });
  assert.equal(storageLoadErrorMessage(offline), DELETE_OFFLINE_MESSAGE);
  assert.equal(storageLoadErrorMessage(new Error('upstream error')), STORAGE_LOAD_ERROR_MESSAGE);
  assert.equal(storageLoadErrorMessage(null), STORAGE_LOAD_ERROR_MESSAGE);
});
