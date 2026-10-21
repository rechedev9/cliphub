import test from 'node:test';
import assert from 'node:assert/strict';
import { projectFreedBytes, projectStorageUsage } from './storage.ts';

const JOB = '11111111-1111-4111-8111-111111111111';

test('projectStorageUsage camelCases the orchestrator body and drops unknown fields', () => {
  const body = {
    jobs: [{ id: JOB, capture_bytes: 10, render_bytes: 5, total_bytes: 20, freeable: true, path: 'C:\\secret' }],
    total_bytes: 20,
    root: 'C:\\secret',
  };
  assert.deepEqual(projectStorageUsage(body), {
    jobs: [{ jobId: JOB, captureBytes: 10, renderBytes: 5, freeable: true }],
    totalBytes: 20,
  });
});

test('projectStorageUsage drops rows without an id and zeroes bad byte counts', () => {
  const body = { jobs: [null, { capture_bytes: 3 }, { id: JOB, capture_bytes: '9', render_bytes: -1, freeable: 'yes' }], total_bytes: 'x' };
  assert.deepEqual(projectStorageUsage(body), {
    jobs: [{ jobId: JOB, captureBytes: 0, renderBytes: 0, freeable: false }],
    totalBytes: 0,
  });
  assert.deepEqual(projectStorageUsage(null), { jobs: [], totalBytes: 0 });
});

test('projectFreedBytes reads freed_bytes, else 0', () => {
  assert.deepEqual(projectFreedBytes({ freed_bytes: 1234 }), { freedBytes: 1234 });
  assert.deepEqual(projectFreedBytes({}), { freedBytes: 0 });
});
