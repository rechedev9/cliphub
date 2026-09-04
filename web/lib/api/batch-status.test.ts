import test from 'node:test';
import assert from 'node:assert/strict';
import { projectBatchStatusItem, type BatchStatusUpstreamItem, type BatchStatusItem } from './batch-status.ts';

const JOB = '11111111-1111-4111-8111-111111111111';

/** Stands in for the route's capture-progress parser. */
const passthroughProgress = (raw: { done?: number; total?: number; percent?: number } | undefined): { done: number; total: number; percent?: number } | undefined => {
  const p = raw;
  if (!p || typeof p.done !== 'number' || typeof p.total !== 'number') return undefined;
  return p.percent === undefined ? { done: p.done, total: p.total } : { done: p.done, total: p.total, percent: p.percent };
};

test('projectBatchStatusItem keeps the fields the client reconciles on', async (t) => {
  const cases: Array<{ name: string; upstream: BatchStatusUpstreamItem; expected: BatchStatusItem }> = [
    {
      // The row that matters. A job the orchestrator could not read arrives
      // with an explicit error and no halves; dropping the error would make
      // the client read it as "job gone" and auto-fire a record intent.
      name: 'a degraded row keeps its explicit error',
      upstream: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: null,
        render: null,
        error: { code: 'render_state_unreadable', message: 'boom' },
      },
      expected: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: null,
        render: null,
        error: { code: 'render_state_unreadable', message: 'boom' },
      },
    },
    {
      name: 'a healthy row carries no error key',
      upstream: { job_id: JOB, variant: 'viral-60-clean', job: { status: 'done' }, render: { status: 'ready' } },
      expected: { job_id: JOB, variant: 'viral-60-clean', job: { status: 'done' }, render: { status: 'ready' } },
    },
    {
      name: 'a missing render becomes null rather than undefined',
      upstream: { job_id: JOB, variant: 'viral-60-clean', job: { status: 'parsed' } },
      expected: { job_id: JOB, variant: 'viral-60-clean', job: { status: 'parsed' }, render: null },
    },
    {
      name: 'failure reason and code survive the projection',
      upstream: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: { status: 'failed', failure_reason: 'demo_incompatible', failure_code: 'parse' },
        render: null,
      },
      expected: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: { status: 'failed', failure_reason: 'demo_incompatible', failure_code: 'parse' },
        render: null,
      },
    },
    {
      name: 'capture progress is carried through the supplied parser',
      upstream: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: { status: 'recording', progress: { done: 3, total: 10, percent: 30 } },
        render: null,
      },
      expected: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: { status: 'recording', progress: { done: 3, total: 10, percent: 30 } },
        render: null,
      },
    },
    {
      name: 'an unknown upstream field is not forwarded',
      upstream: {
        job_id: JOB,
        variant: 'viral-60-clean',
        job: { status: 'done' },
        render: null,
        ...({ demo_path: 'C:/private/match.dem' } as Record<string, unknown>),
      },
      expected: { job_id: JOB, variant: 'viral-60-clean', job: { status: 'done' }, render: null },
    },
  ];

  for (const kase of cases) {
    await t.test(kase.name, () => {
      assert.deepEqual(projectBatchStatusItem(kase.upstream, passthroughProgress), kase.expected);
    });
  }
});

test('projectBatchStatusItem does not invent an error key when there is none', () => {
  const out = projectBatchStatusItem(
    { job_id: JOB, variant: 'viral-60-clean', job: { status: 'done' }, render: null },
    passthroughProgress,
  );
  assert.equal('error' in out, false);
});
