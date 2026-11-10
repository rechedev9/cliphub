import test from 'node:test';
import assert from 'node:assert/strict';
import { povItems, readyVideoJobs } from './pov-jobs.ts';

test('every job is asked about every preset', () => {
  assert.deepEqual(povItems(['j1', 'j2'], ['short', 'wide']), ['j1:short', 'j1:wide', 'j2:short', 'j2:wide']);
});

test('only ready renders with a video count as a POV', () => {
  const jobs = readyVideoJobs([
    { job_id: 'ready', render: { status: 'ready', videos: ['clip'] } },
    { job_id: 'review', render: { status: 'review_required', videos: ['clip'] } },
    { job_id: 'empty', render: { status: 'ready', videos: [] } },
    { job_id: 'running', render: { status: 'rendering', videos: ['clip'] } },
    { job_id: 'broken', error: { code: 'x' }, render: { status: 'ready', videos: ['clip'] } },
    { job_id: 'none', render: null },
    'not a row',
  ]);
  assert.deepEqual([...jobs].sort(), ['ready', 'review']);
});
