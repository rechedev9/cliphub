import test from 'node:test';
import assert from 'node:assert/strict';
import { cloudAccountState, parseCloudAccount, parseCloudJob, parseCloudJobs } from './parse.ts';

const JOB_ID = '11111111-1111-4111-8111-111111111111';
const MATCH_ID = '22222222-2222-4222-8222-222222222222';

const LINKED_ACCOUNT = {
  portal_url: 'https://cliphub.gravityroom.app',
  linked: true,
  link: null,
  user: { name: 'Luis', email: 'luis@example.com' },
  access: 'allowed',
  limits: { max_active: 3, daily_seconds: 5400 },
  usage: { active: 1, seconds_last_24h: 1200, seconds_committed: 480 },
  kinds: ['short'],
  queue: { state: 'online', queued: 4, wait_seconds: { short: 1500 } },
  error: null,
};

const QUEUED_JOB = {
  id: JOB_ID,
  local_job_id: MATCH_ID,
  kind: 'short',
  title: 'R3 4k - Kill Feed',
  status: 'queued',
  stage: null,
  percent: null,
  queue: { position: 3, estimated_start_at: '2026-10-09T10:25:00Z', estimated_done_at: '2026-10-09T10:33:00Z', state: 'online' },
  failure: null,
  cancel_requested: false,
  stalled: null,
  created_at: '2026-10-09T10:00:00Z',
  finished_at: null,
  videos: [],
};

test('a linked account document is read field by field', () => {
  assert.deepEqual(parseCloudAccount(LINKED_ACCOUNT), {
    portalUrl: 'https://cliphub.gravityroom.app',
    linked: true,
    link: null,
    user: { name: 'Luis', email: 'luis@example.com' },
    access: 'allowed',
    limits: { maxActive: 3, dailySeconds: 5400 },
    usage: { active: 1, secondsLast24h: 1200, secondsCommitted: 480 },
    kinds: ['short'],
    queue: { state: 'online', queued: 4, waitSeconds: { short: 1500 } },
    error: null,
  });
});

test('an unlinked account keeps the pending link and nothing that belongs to a user', () => {
  const account = parseCloudAccount({
    portal_url: 'https://cliphub.gravityroom.app',
    linked: false,
    link: { status: 'pending', user_code: 'K7QM-2XHD', verify_url: 'https://cliphub.gravityroom.app/link?code=K7QM-2XHD', expires_at: '2026-10-09T10:10:00Z' },
    user: { name: 'stale', email: 'stale@example.com' },
    access: 'allowed',
    limits: null,
    usage: null,
    kinds: [],
    queue: null,
    error: null,
  });
  assert.equal(account?.linked, false);
  assert.equal(account?.link?.userCode, 'K7QM-2XHD');
  assert.equal(account?.link?.expiresAt, Date.parse('2026-10-09T10:10:00Z'));
  assert.equal(account?.user, null);
  assert.equal(account?.access, null);
});

test('a body that is not an account document is rejected instead of guessed', () => {
  for (const body of [null, 'nope', [], {}, { linked: 'yes' }]) {
    assert.equal(parseCloudAccount(body), null);
  }
});

test('unknown account values degrade to a state that cannot submit or promise a time', () => {
  const account = parseCloudAccount({
    ...LINKED_ACCOUNT,
    access: 'vip',
    queue: { state: 'draining', queued: 2, wait_seconds: { short: 'soon' } },
    error: 'brand_new_error',
  });
  assert.equal(account?.access, null);
  assert.equal(account?.queue?.state, 'offline');
  assert.equal(account?.queue?.waitSeconds.short, null);
  assert.equal(account?.error, 'portal_unreachable');
});

test('the account response tells an old Studio, a disabled client and a dead local service apart', () => {
  assert.equal(cloudAccountState({ status: 200, ok: true, body: LINKED_ACCOUNT }).kind, 'ready');
  assert.equal(cloudAccountState({ status: 404, ok: false, body: { error: 'not found' } }).kind, 'unavailable');
  assert.equal(cloudAccountState({ status: 503, ok: false, body: { error: 'off', code: 'not_configured' } }).kind, 'unavailable');
  assert.equal(
    cloudAccountState({ status: 503, ok: false, body: { error: 'analysis service unavailable', code: 'service_unavailable' } }).kind,
    'offline',
  );
  assert.equal(cloudAccountState({ status: 500, ok: false, body: null }).kind, 'offline');
  assert.equal(cloudAccountState({ status: 200, ok: true, body: '<html>' }).kind, 'unavailable');
});

test('a queued job keeps its position and estimate as epoch milliseconds', () => {
  const job = parseCloudJob(QUEUED_JOB);
  assert.equal(job?.status, 'queued');
  assert.equal(job?.localJobId, MATCH_ID);
  assert.deepEqual(job?.queue, {
    position: 3,
    estimatedStartAt: Date.parse('2026-10-09T10:25:00Z'),
    estimatedDoneAt: Date.parse('2026-10-09T10:33:00Z'),
    state: 'online',
  });
  assert.equal(job?.createdAt, Date.parse('2026-10-09T10:00:00Z'));
});

test('a paused queue never carries an estimate, even if the document has one', () => {
  const job = parseCloudJob({ ...QUEUED_JOB, queue: { ...QUEUED_JOB.queue, state: 'paused' } });
  assert.equal(job?.queue?.state, 'paused');
  assert.equal(job?.queue?.estimatedStartAt, null);
  assert.equal(job?.queue?.position, 3);
});

test('an unknown status is kept as unknown, with no stage, queue or failure invented', () => {
  const job = parseCloudJob({ ...QUEUED_JOB, status: 'teleporting', stage: 'capturing', failure: { code: 'x', message: 'y' } });
  assert.equal(job?.status, 'unknown');
  assert.equal(job?.stage, null);
  assert.equal(job?.queue, null);
  assert.equal(job?.failure, null);
});

test('an unknown stage of a running job is dropped and the percent is clamped', () => {
  const job = parseCloudJob({ ...QUEUED_JOB, status: 'running', stage: 'warming_up', percent: 140, queue: null });
  assert.equal(job?.stage, null);
  assert.equal(job?.percent, 100);
});

test('a failed job always has a failure to show', () => {
  assert.deepEqual(parseCloudJob({ ...QUEUED_JOB, status: 'failed', failure: null })?.failure, { code: '', message: '' });
  assert.deepEqual(
    parseCloudJob({ ...QUEUED_JOB, status: 'failed', failure: { code: 'timeout', message: 'Tardó demasiado' } })?.failure,
    { code: 'timeout', message: 'Tardó demasiado' },
  );
});

test('video names that could not be a URL segment are dropped', () => {
  const job = parseCloudJob({
    ...QUEUED_JOB,
    status: 'done',
    videos: [
      { name: 'short-01.mp4', variant: 'viral-60-clean', size_bytes: 48211332, ready: true },
      { name: '../secret.mp4', variant: 'x', size_bytes: 1, ready: true },
      { name: 'cover.jpg', variant: 'x', size_bytes: 1, ready: true },
      { name: 'short-02.mp4', variant: 'viral-60-clean', size_bytes: 10, ready: 'yes' },
    ],
  });
  assert.deepEqual(job?.videos, [
    { name: 'short-01.mp4', variant: 'viral-60-clean', sizeBytes: 48211332, ready: true },
    { name: 'short-02.mp4', variant: 'viral-60-clean', sizeBytes: 10, ready: false },
  ]);
});

test('the job list skips entries without an id and survives a body that is not a list', () => {
  assert.deepEqual(
    parseCloudJobs({ jobs: [QUEUED_JOB, { status: 'queued' }, null, 'x'] }).map((job) => job.id),
    [JOB_ID],
  );
  assert.deepEqual(parseCloudJobs(null), []);
  assert.deepEqual(parseCloudJobs({ jobs: 'none' }), []);
});

test('why a job is stalled is kept only when this Studio knows the reason', () => {
  assert.equal(parseCloudJob(QUEUED_JOB)?.stalled, null);
  for (const stalled of ['portal_unreachable', 'demo_upload', 'download'] as const) {
    assert.equal(parseCloudJob({ ...QUEUED_JOB, stalled })?.stalled, stalled);
  }
  // An older orchestrator sends no field; a newer one may send a reason this UI cannot word.
  const { stalled: _omitted, ...withoutField } = QUEUED_JOB;
  assert.equal(parseCloudJob(withoutField)?.stalled, null);
  assert.equal(parseCloudJob({ ...QUEUED_JOB, stalled: 'solar_flare' })?.stalled, null);
});
