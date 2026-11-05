import test from 'node:test';
import assert from 'node:assert/strict';
import { projectFreedBytes, projectStorageFiles, projectStorageUsage } from './storage.ts';

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

test('projectStorageFiles camelCases rows and drops malformed ones', () => {
  const body = {
    data_dir: 'C:\\data',
    scanning: true,
    scanned_at: null,
    demos: [
      { path: 'D:\\demos\\a.dem', size_bytes: 9, modified_at: '2026-11-05T10:00:00Z', kind: 'demo', extra: 1 },
      { path: '', kind: 'demo' },
      { path: 'D:\\x.dem', kind: 'unknown' },
      null,
    ],
    studio_demos: [{ path: 'C:\\data\\demos\\j.dem', size_bytes: 'x', kind: 'studio_demo', job_id: JOB }],
  };
  assert.deepEqual(projectStorageFiles(body), {
    dataDir: 'C:\\data',
    scanning: true,
    scannedAt: null,
    demos: [{ path: 'D:\\demos\\a.dem', sizeBytes: 9, modifiedAt: '2026-11-05T10:00:00Z', kind: 'demo' }],
    studioDemos: [{ path: 'C:\\data\\demos\\j.dem', sizeBytes: 0, modifiedAt: '', kind: 'studio_demo', jobId: JOB }],
    videos: [],
  });
});
