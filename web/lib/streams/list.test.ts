import test from 'node:test';
import assert from 'node:assert/strict';
import type { StreamJob } from '../api/streams.ts';
import { sortStreamJobs, streamClipCount, streamJobTag, streamListCadence } from './list.ts';

function job(status: StreamJob['status'], created = '2026-09-01T10:00:00Z'): StreamJob {
  return { id: `job-${status}-${created}`, status, created_at: created };
}

test('each job status maps to one row chip', () => {
  const cases: [StreamJob['status'], string, boolean][] = [
    ['uploaded', 'Borrador', false],
    ['ready', 'Borrador', false],
    ['rendered', 'Renderizado', false],
    ['rendering', 'Render', true],
    ['failed', 'Falló', false],
    ['acquiring', 'Trayendo el vídeo', true],
  ];
  for (const [status, label, busy] of cases) {
    const tag = streamJobTag(job(status));
    assert.equal(tag.label, label);
    assert.equal(tag.busy, busy);
  }
});

test('the list polls fast only while a job is acquiring or rendering', () => {
  assert.equal(streamListCadence([job('ready'), job('rendered')]), 'idle');
  assert.equal(streamListCadence([job('ready'), job('rendering')]), 'fast');
  assert.equal(streamListCadence([job('acquiring')]), 'fast');
  assert.equal(streamListCadence([]), 'idle');
});

test('jobs sort newest first by their last update', () => {
  const older = job('ready', '2026-09-01T10:00:00Z');
  const newer = { ...job('ready', '2026-09-01T09:00:00Z'), updated_at: '2026-09-02T09:00:00Z' };
  assert.deepEqual(
    sortStreamJobs([older, newer]).map((entry) => entry.id),
    [newer.id, older.id],
  );
});

test('streamClipCount prefers the list row count and falls back to the plan', () => {
  assert.equal(streamClipCount({ clip_count: 3 }), 3);
  assert.equal(streamClipCount({ clip_count: 0, edit_plan: { schema_version: '1', variant: 'streamer-fullframe-nocam', clips: [{ id: 'c1', start_seconds: 0, end_seconds: 5 }] } }), 0);
  assert.equal(streamClipCount({ edit_plan: { schema_version: '1', variant: 'streamer-fullframe-nocam', clips: [{ id: 'c1', start_seconds: 0, end_seconds: 5 }] } }), 1);
  assert.equal(streamClipCount({}), 0);
});
