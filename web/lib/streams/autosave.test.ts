import test from 'node:test';
import assert from 'node:assert/strict';
import { shouldAutosaveStreamPlan, type AckedStreamPlanFingerprint } from './autosave.ts';

test('autosave PUTs only when the plan differs from the last server-acknowledged revision', () => {
  const acked: AckedStreamPlanFingerprint = { jobId: 'job-1', fingerprint: 'fp-a' };
  const cases: [string, string, string, AckedStreamPlanFingerprint, boolean, boolean][] = [
    ['no acknowledgement yet (initial load, or before any PUT lands)', 'job-1', 'fp-a', null, false, true],
    ['same job, unchanged fingerprint: skip the redundant PUT', 'job-1', 'fp-a', acked, false, false],
    ['same job, changed fingerprint: autosave', 'job-1', 'fp-b', acked, false, true],
    ['different job, even with a matching fingerprint: autosave', 'job-2', 'fp-a', acked, false, true],
    ['unchanged fingerprint but a PUT in flight: its ack is for an older plan, so autosave', 'job-1', 'fp-a', acked, true, true],
  ];
  for (const [label, jobId, fingerprint, ackedState, inFlight, expected] of cases) {
    assert.equal(shouldAutosaveStreamPlan(jobId, fingerprint, ackedState, inFlight), expected, label);
  }
});
