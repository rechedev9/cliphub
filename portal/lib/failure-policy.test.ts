import { test } from "node:test";
import assert from "node:assert/strict";

import { classOf, decideFailure, type FailureInput } from "./failure-policy.ts";
import { FAILURE_CODES } from "./job-types.ts";

const FIRST_ATTEMPT = { attempt: 1, maxAttempts: 2, cancelRequestedAt: null };
const LAST_ATTEMPT = { attempt: 2, maxAttempts: 2, cancelRequestedAt: null };

function decide(input: Partial<FailureInput> & Pick<FailureInput, "code">) {
  const decision = decideFailure({ job: FIRST_ATTEMPT, source: "worker", ...input });
  assert.equal(decision.ok, true);
  if (!decision.ok) throw new Error("unreachable");
  return decision;
}

test("every failure code has the class the contract gives it", () => {
  const classes = Object.fromEntries(FAILURE_CODES.map((code) => [code, classOf(code)]));
  assert.deepEqual(classes, {
    capture_flake: "retry",
    interrupted: "retry",
    worker_lost: "retry",
    demo_download_failed: "retry",
    internal: "retry",
    demo_incompatible: "terminal",
    unplayable_start: "terminal",
    target_not_found: "terminal",
    spec_mismatch: "terminal",
    invalid_spec: "terminal",
    render_failed: "terminal",
    job_failed: "terminal",
    timeout: "terminal",
    upload_failed: "terminal",
    capture_incompatible: "machine",
    steam_unavailable: "machine",
    disk_full: "machine",
    tools_missing: "machine",
    canceled: "cancel",
  });
});

test("a retryable failure requeues while attempts remain", () => {
  const decision = decide({ code: "capture_flake" });
  assert.equal(decision.outcome, "requeued");
  assert.equal(decision.status, "queued");
  assert.equal(decision.attempt, 1);
  assert.equal(decision.pauseReason, null);
});

test("a retryable failure on the last attempt fails the job", () => {
  const decision = decide({ job: LAST_ATTEMPT, code: "capture_flake" });
  assert.equal(decision.outcome, "failed");
  assert.equal(decision.status, "failed");
});

test("a terminal failure fails at once, even on the first attempt", () => {
  const decision = decide({ code: "spec_mismatch" });
  assert.equal(decision.outcome, "failed");
  assert.equal(decision.consecutiveFailures, 0);
});

test("a machine fault requeues, gives the attempt back and pauses the worker", () => {
  const decision = decide({
    code: "capture_incompatible",
    detail: "HLAE hook crashed (AfxHookSource2)",
  });
  assert.equal(decision.outcome, "requeued");
  assert.equal(decision.attempt, 0);
  assert.equal(decision.pauseReason, "capture_incompatible: HLAE hook crashed (AfxHookSource2)");
  assert.equal(decision.consecutiveFailures, 0);
});

test("a machine fault on the last attempt still requeues instead of failing", () => {
  const decision = decide({ job: LAST_ATTEMPT, code: "disk_full" });
  assert.equal(decision.outcome, "requeued");
  assert.equal(decision.attempt, 1);
});

test("the pause reason keeps at most 300 characters of detail", () => {
  const decision = decide({ code: "tools_missing", detail: "x".repeat(1000) });
  assert.equal(decision.pauseReason, `tools_missing: ${"x".repeat(300)}`);
});

test("the cancel code is only valid when a cancel was requested", () => {
  assert.deepEqual(
    decideFailure({ job: FIRST_ATTEMPT, code: "canceled", source: "worker" }),
    { ok: false },
  );
  const decision = decide({ job: { ...FIRST_ATTEMPT, cancelRequestedAt: 5 }, code: "canceled" });
  assert.equal(decision.outcome, "canceled");
  assert.equal(decision.status, "canceled");
});

test("a job with a pending cancel is canceled instead of requeued", () => {
  const pending = { ...FIRST_ATTEMPT, cancelRequestedAt: 5 };
  assert.equal(decide({ job: pending, code: "interrupted" }).outcome, "canceled");
  assert.equal(decide({ job: pending, code: "worker_lost", source: "lease" }).outcome, "canceled");
  const machine = decide({ job: pending, code: "steam_unavailable" });
  assert.equal(machine.outcome, "canceled");
  assert.notEqual(machine.pauseReason, null);
});

test("a pending cancel does not hide a terminal failure", () => {
  const pending = { ...FIRST_ATTEMPT, cancelRequestedAt: 5 };
  assert.equal(decide({ job: pending, code: "render_failed" }).outcome, "failed");
});

test("the breaker pauses the worker at the third consecutive failure and resets", () => {
  const second = decide({ code: "capture_flake", consecutiveFailures: 1 });
  assert.equal(second.consecutiveFailures, 2);
  assert.equal(second.pauseReason, null);

  const third = decide({ code: "job_failed", consecutiveFailures: 2, detail: "cs2 never started" });
  assert.equal(third.consecutiveFailures, 0);
  assert.equal(third.pauseReason, "consecutive_failures: cs2 never started");
  assert.equal(third.outcome, "failed");
});

test("deterministic failures other than job_failed do not move the breaker", () => {
  assert.equal(decide({ code: "spec_mismatch", consecutiveFailures: 2 }).consecutiveFailures, 2);
  assert.equal(decide({ code: "disk_full", consecutiveFailures: 2 }).consecutiveFailures, 2);
});

test("an expired lease does not count against the worker's breaker", () => {
  const decision = decide({ code: "worker_lost", source: "lease", consecutiveFailures: 2 });
  assert.equal(decision.consecutiveFailures, 2);
  assert.equal(decision.pauseReason, null);
  assert.equal(decision.outcome, "requeued");
});

test("a lost lease with a cancel pending is canceled even on the last attempt", () => {
  const pending = { ...LAST_ATTEMPT, cancelRequestedAt: 5 };
  const lost = decide({ job: pending, code: "worker_lost", source: "lease" });
  assert.equal(lost.outcome, "canceled");
  assert.equal(lost.status, "canceled");
  // The worker itself reporting a retryable failure on the last attempt is still a failure.
  assert.equal(decide({ job: pending, code: "capture_flake" }).outcome, "failed");
});

test("the third machine fault of the same job fails it for good and pauses nobody", () => {
  const first = decide({ code: "capture_incompatible" });
  assert.equal(first.machineRequeues, 1);
  const second = decide({ job: { ...FIRST_ATTEMPT, machineRequeues: 1 }, code: "capture_incompatible" });
  assert.equal(second.outcome, "requeued");
  assert.equal(second.machineRequeues, 2);

  const third = decide({ job: { ...FIRST_ATTEMPT, machineRequeues: 2 }, code: "capture_incompatible" });
  assert.equal(third.outcome, "failed");
  assert.equal(third.status, "failed");
  assert.equal(third.attempt, 1);
  assert.equal(third.pauseReason, null);
});

test("an automatic pause names the job that caused it", () => {
  const job = { ...FIRST_ATTEMPT, id: "job-1", title: 'R3 "ace"' };
  assert.equal(
    decide({ job, code: "disk_full", detail: "2 GB left" }).pauseReason,
    "disk_full: job job-1 \"R3 'ace'\": 2 GB left",
  );
  assert.equal(
    decide({ job, code: "capture_flake", consecutiveFailures: 2, detail: "cs2 timeout" }).pauseReason,
    "consecutive_failures: job job-1 \"R3 'ace'\": cs2 timeout",
  );
});
