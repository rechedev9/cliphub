import {
  DEMO_SHA,
  MINUTE,
  NOW,
  eventTypes,
  getJob,
  getWorker,
  makeQueuedJob,
  makeUser,
  makeWorker,
  resetDatabase,
} from "./test-support.ts";

import { beforeEach, test } from "node:test";
import assert from "node:assert/strict";
import { rm } from "node:fs/promises";

import { eq } from "drizzle-orm";

import { db } from "../db/client.ts";
import { requestArtifacts, requests, userCloud, workers } from "../db/schema.ts";
import { demoFileSize } from "./demo-store.ts";
import {
  cancelJob,
  claimNext,
  completeJob,
  enterUploading,
  failJob,
  queueSnapshot,
  recordHeartbeat,
  retryJob,
  setPriority,
  sweepLeases,
  type WorkerRow,
} from "./queue-store.ts";
import { parseWorkerHealth, type FailBody, type HeartbeatBody } from "./worker-protocol.ts";

const LEASE_MS = 90_000;
const UPLOAD_GRACE_MS = 30 * MINUTE;

beforeEach(resetDatabase);

async function claim(worker: WorkerRow, now = NOW) {
  return claimNext({ worker, kinds: ["short"], now });
}

async function claimedId(worker: WorkerRow, now = NOW): Promise<string> {
  const result = await claim(worker, now);
  if (!result.claimed) throw new Error(`nothing claimed: ${result.reason}`);
  return result.job.id;
}

function failure(code: FailBody["code"], detail = "raw cause"): FailBody {
  return { code, message: "Mensaje para el usuario.", detail, machineSeconds: 120 };
}

// What the worker that claimed the job last would send: the job's current attempt.
async function held(worker: WorkerRow, jobId: string) {
  return { worker, jobId, attempt: (await getJob(jobId)).attempt };
}

function heartbeat(jobs: HeartbeatBody["jobs"]): HeartbeatBody {
  return { state: "busy", blocked: null, health: parseWorkerHealth({}), jobs };
}

async function toUploading(worker: WorkerRow, jobId: string): Promise<void> {
  const entered = await enterUploading({
    ...(await held(worker, jobId)),
    body: { machineSeconds: 400, localJobId: "local-1" },
    now: NOW,
  });
  assert.equal(entered, true);
}

async function addReadyVideo(jobId: string): Promise<void> {
  await db.insert(requestArtifacts).values({
    requestId: jobId,
    variant: "viral-60-clean",
    name: "short-01.mp4",
    path: `missing-${jobId}.bin`,
    sizeBytes: 10,
    sha256: DEMO_SHA,
    status: "ready",
  });
}

test("a claim on an empty queue says so", async () => {
  const worker = await makeWorker();
  assert.deepEqual(await claim(worker), { claimed: false, reason: "empty" });
});

test("a claim hands out the job with its spec and demo, and leases it for 90 seconds", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("luis"), estimatedSeconds: 480 });

  const result = await claim(worker);
  assert.equal(result.claimed, true);
  if (!result.claimed) return;
  assert.equal(result.job.id, jobId);
  assert.equal(result.job.attempt, 1);
  assert.equal(result.job.leaseExpiresAt, NOW + LEASE_MS);
  assert.equal(result.job.maxRuntimeSeconds, 1440);
  assert.deepEqual(result.job.demo, { sha256: DEMO_SHA, sizeBytes: 18, fileName: "match.dem" });
  assert.equal(result.job.submitterLabel, "luis");
  assert.equal(result.job.spec.targetSteamId, "76561198000000001");

  const job = await getJob(jobId);
  assert.equal(job.status, "running");
  assert.equal(job.stage, "downloading");
  assert.equal(job.workerId, worker.id);
  assert.equal(job.claimedAt?.getTime(), NOW);
});

test("a worker with a running job is refused a second one", async () => {
  const worker = await makeWorker();
  const userId = await makeUser("a");
  await makeQueuedJob({ userId });
  await makeQueuedJob({ userId: await makeUser("b") });

  await claimedId(worker);
  assert.deepEqual(await claim(worker), { claimed: false, reason: "busy" });
});

test("two workers racing for one job: exactly one gets it", async () => {
  const first = await makeWorker("one");
  const second = await makeWorker("two");
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });

  const results = await Promise.all([claim(first), claim(second)]);
  const winners = results.filter((result) => result.claimed);
  assert.equal(winners.length, 1);
  assert.equal((await getJob(jobId)).attempt, 1);
  assert.deepEqual(
    results.find((result) => !result.claimed),
    { claimed: false, reason: "empty" },
  );
});

test("a paused worker and a full portal volume get no job, and say why", async () => {
  const worker = await makeWorker();
  await makeQueuedJob({ userId: await makeUser("a") });

  await db.update(workers).set({ paused: true }).where(eq(workers.id, worker.id));
  assert.deepEqual(await claim(worker), { claimed: false, reason: "paused" });
  await db.update(workers).set({ paused: false }).where(eq(workers.id, worker.id));

  process.env.MIN_FREE_BYTES_CLAIM = String(Number.MAX_SAFE_INTEGER);
  try {
    assert.deepEqual(await claim(worker), { claimed: false, reason: "portal_storage_full" });
  } finally {
    process.env.MIN_FREE_BYTES_CLAIM = "1";
  }
});

test("a volume too full for new jobs still lets the worker claim, because finishing jobs frees demos", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  process.env.MIN_FREE_BYTES = String(Number.MAX_SAFE_INTEGER);
  try {
    assert.equal(await claimedId(worker), jobId);
  } finally {
    process.env.MIN_FREE_BYTES = "1";
  }
});

test("a worker that does not take the kind, and a blocked user, leave the job queued", async () => {
  const worker = await makeWorker();
  const userId = await makeUser("a");
  const jobId = await makeQueuedJob({ userId });

  const noKinds = await claimNext({ worker, kinds: [], now: NOW });
  assert.deepEqual(noKinds, { claimed: false, reason: "empty" });

  await db.insert(userCloud).values({ userId, access: "blocked", updatedAt: NOW });
  assert.deepEqual(await claim(worker), { claimed: false, reason: "empty" });
  assert.equal((await getJob(jobId)).status, "queued");
});

test("a user who just used the machine yields to another user who waited about as long", async () => {
  const worker = await makeWorker();
  const alice = await makeUser("alice");
  const bob = await makeUser("bob");
  const a1 = await makeQueuedJob({ userId: alice, enqueuedAt: NOW - 30 * MINUTE });
  const a2 = await makeQueuedJob({ userId: alice, enqueuedAt: NOW - 29 * MINUTE });
  const b1 = await makeQueuedJob({ userId: bob, enqueuedAt: NOW - 28 * MINUTE });

  assert.equal(await claimedId(worker), a1);
  await toUploading(worker, a1);
  assert.equal(await claimedId(worker), b1);
  await toUploading(worker, b1);
  assert.equal(await claimedId(worker), a2);
});

test("a job moved to the front is taken first, and reset gives its place back", async () => {
  const worker = await makeWorker();
  const old = await makeQueuedJob({ userId: await makeUser("a"), enqueuedAt: NOW - 60 * MINUTE });
  const fresh = await makeQueuedJob({ userId: await makeUser("b"), enqueuedAt: NOW - MINUTE });
  const admin = { adminUserId: "admin-1", now: NOW };

  assert.equal(await setPriority({ jobId: fresh, action: "front", ...admin }), "ok");
  assert.equal((await queueSnapshot(NOW)).placements.get(fresh)?.position, 1);

  assert.equal(await setPriority({ jobId: fresh, action: "reset", ...admin }), "ok");
  assert.equal((await queueSnapshot(NOW)).placements.get(old)?.position, 1);

  await setPriority({ jobId: fresh, action: "front", ...admin });
  assert.equal(await claimedId(worker), fresh);
  assert.equal(await setPriority({ jobId: fresh, action: "front", ...admin }), "invalid_state");
});

test("a heartbeat extends the lease, records progress and reports jobs it does not hold as lost", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);

  const later = NOW + 15_000;
  const result = await recordHeartbeat({
    worker,
    now: later,
    body: heartbeat([
      { id: jobId, phase: "running", attempt: 1, stage: "capturing", percent: 62, detail: "REC 3/5", localJobId: "local-9" },
      { id: "not-mine", phase: "running", attempt: 1, stage: null, percent: null, detail: null, localJobId: null },
    ]),
  });

  assert.deepEqual(result.jobs, [
    { id: jobId, lease: "ok", leaseExpiresAt: later + LEASE_MS, cancelRequested: false },
    { id: "not-mine", lease: "lost", leaseExpiresAt: 0, cancelRequested: false },
  ]);
  const job = await getJob(jobId);
  assert.equal(job.leaseExpiresAt, later + LEASE_MS);
  assert.equal(job.stage, "capturing");
  assert.equal(job.progressPercent, 62);
  assert.equal(job.localJobId, "local-9");
  assert.equal((await getWorker(worker.id)).lastSeenAt, later);
  assert.deepEqual(await eventTypes(jobId), ["claimed", "stage"]);
});

test("a heartbeat cannot keep another worker's job alive", async () => {
  const owner = await makeWorker("owner");
  const intruder = await makeWorker("intruder");
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(owner);

  const result = await recordHeartbeat({
    worker: intruder,
    now: NOW + 15_000,
    body: heartbeat([
      { id: jobId, phase: "running", attempt: 1, stage: "rendering", percent: 99, detail: null, localJobId: null },
    ]),
  });
  assert.equal(result.jobs[0]?.lease, "lost");
  const job = await getJob(jobId);
  assert.equal(job.leaseExpiresAt, NOW + LEASE_MS);
  assert.equal(job.stage, "downloading");
});

test("a heartbeat tells the worker when it has been paused", async () => {
  const worker = await makeWorker();
  await db
    .update(workers)
    .set({ paused: true, pausedBy: "admin", pauseReason: "mantenimiento" })
    .where(eq(workers.id, worker.id));
  const result = await recordHeartbeat({ worker, now: NOW, body: heartbeat([]) });
  assert.equal(result.paused, true);
  assert.equal(result.pauseReason, "mantenimiento");
});

test("a lease is not swept before it expires", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);

  assert.equal(await sweepLeases(NOW + LEASE_MS), 0);
  assert.equal((await getJob(jobId)).status, "running");
});

test("an expired lease requeues the job with its original seniority, then fails it on the last attempt", async () => {
  const worker = await makeWorker();
  const enqueuedAt = NOW - 20 * MINUTE;
  const jobId = await makeQueuedJob({ userId: await makeUser("a"), enqueuedAt });
  await claimedId(worker);

  assert.equal(await sweepLeases(NOW + LEASE_MS + 1), 1);
  let job = await getJob(jobId);
  assert.equal(job.status, "queued");
  assert.equal(job.attempt, 1);
  assert.equal(job.enqueuedAt, enqueuedAt);
  assert.equal(job.workerId, null);
  assert.equal(job.leaseExpiresAt, null);
  assert.equal(job.stage, null);

  const second = NOW + 5 * MINUTE;
  assert.equal(await claimedId(worker, second), jobId);
  assert.equal(await sweepLeases(second + LEASE_MS + 1), 1);
  job = await getJob(jobId);
  assert.equal(job.status, "failed");
  assert.equal(job.attempt, 2);
  assert.equal(job.failureCode, "worker_lost");
  assert.equal(job.finishedAt, second + LEASE_MS + 1);
  assert.ok(job.failureReason && job.failureReason.length > 0);
  assert.deepEqual(await eventTypes(jobId), [
    "claimed",
    "lease_expired",
    "requeued",
    "claimed",
    "lease_expired",
    "failed",
  ]);
});

test("a claim sweeps expired leases first, so a dead worker's job can be taken at once", async () => {
  const dead = await makeWorker("dead");
  const alive = await makeWorker("alive");
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(dead);

  const later = NOW + 2 * MINUTE;
  const result = await claim(alive, later);
  assert.equal(result.claimed, true);
  if (result.claimed) assert.equal(result.job.attempt, 2);
  assert.equal((await getJob(jobId)).workerId, alive.id);
});

test("an uploading job gets 30 more minutes before its lease is lost", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  await toUploading(worker, jobId);
  const expiry = NOW + LEASE_MS;

  assert.equal(await sweepLeases(expiry + UPLOAD_GRACE_MS), 0);
  assert.equal((await getJob(jobId)).status, "uploading");

  assert.equal(await sweepLeases(expiry + UPLOAD_GRACE_MS + 1), 1);
  assert.equal((await getJob(jobId)).status, "queued");
});

test("a lost lease with a pending cancel becomes canceled, not a retry", async () => {
  const worker = await makeWorker();
  const userId = await makeUser("a");
  const jobId = await makeQueuedJob({ userId });
  await claimedId(worker);
  await cancelJob({ jobId, by: "user", actorUserId: userId, now: NOW + 1000 });

  await sweepLeases(NOW + LEASE_MS + 1);
  const job = await getJob(jobId);
  assert.equal(job.status, "canceled");
  assert.equal(job.canceledBy, "user");
});

test("moving to uploading frees the capture slot and adds the machine time", async () => {
  const worker = await makeWorker();
  await makeQueuedJob({ userId: await makeUser("a") });
  await makeQueuedJob({ userId: await makeUser("b") });
  const first = await claimedId(worker);
  await toUploading(worker, first);

  const job = await getJob(first);
  assert.equal(job.status, "uploading");
  assert.equal(job.stage, null);
  assert.equal(job.machineSeconds, 400);
  assert.equal(job.localJobId, "local-1");
  assert.equal((await claim(worker)).claimed, true);
});

test("repeating the uploading call is harmless, but another worker is told the lease is lost", async () => {
  const worker = await makeWorker();
  const other = await makeWorker("other");
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  const body = { machineSeconds: 400, localJobId: null };

  assert.equal(await enterUploading({ ...(await held(other, jobId)), body, now: NOW }), false);
  assert.equal(await enterUploading({ ...(await held(worker, jobId)), body, now: NOW }), true);
  assert.equal(await enterUploading({ ...(await held(worker, jobId)), body, now: NOW }), true);
  assert.equal((await getJob(jobId)).machineSeconds, 400);
});

test("a job cannot be completed without a finished video", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);

  assert.equal(await completeJob({ ...(await held(worker, jobId)), now: NOW }), "lease_lost");
  await toUploading(worker, jobId);
  assert.equal(await completeJob({ ...(await held(worker, jobId)), now: NOW }), "no_artifacts");
  assert.equal((await getJob(jobId)).status, "uploading");
});

test("completing a job marks it done and clears the worker's failure streak", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await db.update(workers).set({ consecutiveFailures: 2 }).where(eq(workers.id, worker.id));
  await claimedId(worker);
  await toUploading(worker, jobId);
  await addReadyVideo(jobId);

  assert.equal(await completeJob({ ...(await held(worker, jobId)), now: NOW + 5000 }), "ok");
  const job = await getJob(jobId);
  assert.equal(job.status, "done");
  assert.equal(job.finishedAt, NOW + 5000);
  assert.equal((await getWorker(worker.id)).consecutiveFailures, 0);
  assert.equal(await completeJob({ ...(await held(worker, jobId)), now: NOW + 6000 }), "ok");
});

test("a deterministic failure fails the job at once with the user message and the raw cause", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);

  const result = await failJob({ ...(await held(worker, jobId)), now: NOW + 1000, body: failure("spec_mismatch", "seg-001 moved") });
  assert.deepEqual(result, { outcome: "failed", workerPaused: false });
  const job = await getJob(jobId);
  assert.equal(job.status, "failed");
  assert.equal(job.failureCode, "spec_mismatch");
  assert.equal(job.failureReason, "Mensaje para el usuario.");
  assert.equal(job.failureDetail, "seg-001 moved");
  assert.equal(job.machineSeconds, 120);
});

test("a machine fault requeues the job at attempt 0 and auto-pauses the worker", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);

  const result = await failJob({
    ...(await held(worker, jobId)),
    now: NOW + 1000,
    body: failure("capture_incompatible", "AfxHookSource2 crashed"),
  });
  assert.deepEqual(result, { outcome: "requeued", workerPaused: true });

  const job = await getJob(jobId);
  assert.equal(job.status, "queued");
  assert.equal(job.attempt, 0);
  assert.equal(job.machineSeconds, 0);
  const paused = await getWorker(worker.id);
  assert.equal(paused.paused, true);
  assert.equal(paused.pausedBy, "auto");
  assert.equal(paused.pauseReason, `capture_incompatible: job ${jobId} "R3 4k": AfxHookSource2 crashed`);
  assert.deepEqual(await claim(worker), { claimed: false, reason: "paused" });
  assert.deepEqual(await eventTypes(jobId), ["claimed", "requeued", "worker_auto_paused"]);
});

test("three retryable failures in a row pause the worker", async () => {
  const worker = await makeWorker();
  const oldest = await makeQueuedJob({ userId: await makeUser("a"), enqueuedAt: NOW - 10 * MINUTE });
  const newer = await makeQueuedJob({ userId: await makeUser("b"), enqueuedAt: NOW - MINUTE });

  const outcomes = [];
  const claimed = [];
  for (let attempt = 0; attempt < 3; attempt += 1) {
    const jobId = await claimedId(worker);
    claimed.push(jobId);
    outcomes.push(await failJob({ ...(await held(worker, jobId)), now: NOW, body: failure("capture_flake", "cs2 timeout") }));
  }
  // The oldest job keeps its seniority, so it is retried first and uses up its two attempts.
  assert.deepEqual(claimed, [oldest, oldest, newer]);
  assert.deepEqual(outcomes, [
    { outcome: "requeued", workerPaused: false },
    { outcome: "failed", workerPaused: false },
    { outcome: "requeued", workerPaused: true },
  ]);
  const paused = await getWorker(worker.id);
  assert.equal(paused.pauseReason, `consecutive_failures: job ${newer} "R3 4k": cs2 timeout`);
  assert.equal(paused.consecutiveFailures, 0);
});

test("a failure from a worker that does not hold the job changes nothing", async () => {
  const owner = await makeWorker("owner");
  const other = await makeWorker("other");
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(owner);

  assert.equal(await failJob({ ...(await held(other, jobId)), now: NOW, body: failure("job_failed") }), "lease_lost");
  assert.equal((await getJob(jobId)).status, "running");
  assert.equal(await failJob({ ...(await held(owner, jobId)), now: NOW, body: failure("canceled") }), "invalid");
});

test("failing an uploading job discards the files of that attempt", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  await toUploading(worker, jobId);
  await addReadyVideo(jobId);

  await failJob({ ...(await held(worker, jobId)), now: NOW, body: failure("upload_failed") });
  const left = await db.select().from(requestArtifacts).where(eq(requestArtifacts.requestId, jobId));
  assert.equal(left.length, 0);
});

test("cancelling a queued job takes effect at once", async () => {
  const userId = await makeUser("a");
  const jobId = await makeQueuedJob({ userId });

  const result = await cancelJob({ jobId, by: "user", actorUserId: userId, now: NOW + 1000 });
  assert.deepEqual(result, { status: "canceled", cancelRequested: false });
  const job = await getJob(jobId);
  assert.equal(job.status, "canceled");
  assert.equal(job.finishedAt, NOW + 1000);
  assert.equal(await cancelJob({ jobId, by: "user", actorUserId: userId, now: NOW }), "invalid_state");
});

test("cancelling a running job asks the worker, which confirms with the cancel code", async () => {
  const worker = await makeWorker();
  const userId = await makeUser("a");
  const jobId = await makeQueuedJob({ userId });
  await claimedId(worker);

  const requested = await cancelJob({ jobId, by: "admin", actorUserId: "admin-1", now: NOW + 1000 });
  assert.deepEqual(requested, { status: "running", cancelRequested: true });
  assert.equal((await getJob(jobId)).status, "running");

  const beat = await recordHeartbeat({
    worker,
    now: NOW + 2000,
    body: heartbeat([{ id: jobId, phase: "running", attempt: 1, stage: null, percent: null, detail: null, localJobId: null }]),
  });
  assert.equal(beat.jobs[0]?.cancelRequested, true);

  const result = await failJob({ ...(await held(worker, jobId)), now: NOW + 3000, body: failure("canceled") });
  assert.deepEqual(result, { outcome: "canceled", workerPaused: false });
  const job = await getJob(jobId);
  assert.equal(job.status, "canceled");
  assert.equal(job.canceledBy, "admin");
});

test("an operator can cancel a legacy manual request, a user cannot", async () => {
  const userId = await makeUser("a");
  const [legacy] = await db
    .insert(requests)
    .values({ userId, status: "pending", note: "old request" })
    .returning({ id: requests.id });
  if (!legacy) throw new Error("legacy row not created");

  assert.equal(await cancelJob({ jobId: legacy.id, by: "user", actorUserId: userId, now: NOW }), "invalid_state");
  assert.deepEqual(await cancelJob({ jobId: legacy.id, by: "admin", actorUserId: "admin-1", now: NOW }), {
    status: "canceled",
    cancelRequested: false,
  });
});

test("an operator retry requeues a failed job with a fresh attempt count and its seniority", async () => {
  const worker = await makeWorker();
  const enqueuedAt = NOW - 40 * MINUTE;
  const jobId = await makeQueuedJob({ userId: await makeUser("a"), enqueuedAt });
  const admin = { adminUserId: "admin-1", now: NOW + 5000 };

  assert.equal(await retryJob({ jobId, ...admin }), "invalid_state");
  await claimedId(worker);
  await failJob({ ...(await held(worker, jobId)), now: NOW, body: failure("render_failed") });

  assert.equal(await retryJob({ jobId, ...admin }), "ok");
  const job = await getJob(jobId);
  assert.equal(job.status, "queued");
  assert.equal(job.attempt, 0);
  assert.equal(job.enqueuedAt, enqueuedAt);
  assert.equal(job.failureCode, null);
  assert.equal(job.failureReason, null);
  assert.equal(job.finishedAt, null);
  assert.equal(await retryJob({ jobId: "missing", ...admin }), "not_found");
});

test("a retry is refused once the demo is gone", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  await failJob({ ...(await held(worker, jobId)), now: NOW, body: failure("render_failed") });
  await rm((await getJob(jobId)).demoPath ?? "", { force: true });

  assert.equal(await retryJob({ jobId, adminUserId: "admin-1", now: NOW }), "demo_gone");
  assert.equal((await getJob(jobId)).status, "failed");
});

test("the snapshot gives no start times while no worker is online", async () => {
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });

  const offline = await queueSnapshot(NOW);
  assert.equal(offline.state, "offline");
  assert.deepEqual(offline.placements.get(jobId), {
    position: 1,
    estimatedStartAt: null,
    estimatedDoneAt: null,
  });

  await makeWorker();
  // Still the cached answer inside its 5 seconds, then a fresh one.
  assert.equal((await queueSnapshot(NOW + 4000)).state, "offline");
  const online = await queueSnapshot(NOW + 5000);
  assert.equal(online.state, "online");
  assert.equal(online.placements.get(jobId)?.estimatedStartAt, NOW + 5000);
});

const SHORT_LIMIT_MS = (1440 + 300) * 1000;
const UPLOAD_LIMIT_MS = 6.5 * 60 * MINUTE;

function entry(jobId: string, overrides: Partial<HeartbeatBody["jobs"][number]> = {}) {
  return {
    id: jobId,
    phase: "running" as const,
    attempt: 1,
    stage: null,
    percent: null,
    detail: null,
    localJobId: null,
    ...overrides,
  };
}

test("a claim never takes back the claiming worker's own expired upload", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  await toUploading(worker, jobId);
  await addReadyVideo(jobId);

  // The worker was unreachable for longer than the lease plus the upload grace; its claim lands first.
  const back = NOW + LEASE_MS + UPLOAD_GRACE_MS + 2 * MINUTE;
  assert.deepEqual(await claim(worker, back), { claimed: false, reason: "empty" });
  const job = await getJob(jobId);
  assert.equal(job.status, "uploading");
  assert.equal(job.attempt, 1);
  const files = await db.select().from(requestArtifacts).where(eq(requestArtifacts.requestId, jobId));
  assert.equal(files.length, 1);

  // Its next heartbeat keeps the upload, and the finished video is delivered.
  const beat = await recordHeartbeat({
    worker,
    now: back + 1000,
    body: heartbeat([entry(jobId, { phase: "uploading" })]),
  });
  assert.equal(beat.jobs[0]?.lease, "ok");
  assert.equal(await completeJob({ worker, jobId, attempt: 1, now: back + 2000 }), "ok");
});

test("another worker's claim still sweeps an expired upload", async () => {
  const dead = await makeWorker("dead");
  const alive = await makeWorker("alive");
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(dead);
  await toUploading(dead, jobId);

  const later = NOW + LEASE_MS + UPLOAD_GRACE_MS + MINUTE;
  const result = await claim(alive, later);
  assert.equal(result.claimed && result.job.id === jobId && result.job.attempt === 2, true);
});

test("calls from the attempt that lost the job are refused once the job is on a new attempt", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  await toUploading(worker, jobId);

  // The timer sweeps while the worker is away; on its return the same worker claims the job again.
  const back = NOW + 40 * MINUTE;
  assert.equal(await sweepLeases(back), 1);
  const again = await claim(worker, back);
  assert.equal(again.claimed && again.job.attempt === 2, true);
  const stale = { worker, jobId, attempt: 1 };
  const current = { worker, jobId, attempt: 2 };

  const beat = await recordHeartbeat({
    worker,
    now: back + 1000,
    body: heartbeat([entry(jobId, { phase: "uploading", localJobId: "local-old" })]),
  });
  assert.deepEqual(beat.jobs, [{ id: jobId, lease: "lost", leaseExpiresAt: 0, cancelRequested: false }]);
  assert.equal((await getJob(jobId)).leaseExpiresAt, back + LEASE_MS);

  const body = { machineSeconds: 9999, localJobId: "local-old" };
  assert.equal(await enterUploading({ ...stale, body, now: back + 2000 }), false);
  assert.equal(await completeJob({ ...stale, now: back + 2000 }), "lease_lost");
  assert.equal(await failJob({ ...stale, now: back + 2000, body: failure("upload_failed") }), "lease_lost");

  const job = await getJob(jobId);
  assert.equal(job.status, "running");
  assert.equal(job.attempt, 2);
  assert.equal(job.machineSeconds, 400);
  assert.equal(job.localJobId, null);

  const fresh = await recordHeartbeat({
    worker,
    now: back + 3000,
    body: heartbeat([entry(jobId, { attempt: 2 })]),
  });
  assert.equal(fresh.jobs[0]?.lease, "ok");
  assert.equal(await enterUploading({ ...current, body, now: back + 4000 }), true);
});

test("a heartbeat entry that names no attempt is answered as lost and renews nothing", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);

  const beat = await recordHeartbeat({
    worker,
    now: NOW + 15_000,
    body: heartbeat([entry(jobId, { attempt: null, stage: "capturing", percent: 50 })]),
  });
  assert.equal(beat.jobs[0]?.lease, "lost");
  const job = await getJob(jobId);
  assert.equal(job.leaseExpiresAt, NOW + LEASE_MS);
  assert.equal(job.stage, "downloading");
  assert.equal((await getWorker(worker.id)).lastSeenAt, NOW + 15_000);
});

test("a worker holding three uploads is refused new claims until one ends", async () => {
  const worker = await makeWorker();
  const uploading: string[] = [];
  for (const name of ["a", "b", "c", "d"]) {
    await makeQueuedJob({ userId: await makeUser(name) });
  }
  for (let index = 0; index < 3; index += 1) {
    const jobId = await claimedId(worker);
    await toUploading(worker, jobId);
    uploading.push(jobId);
  }

  assert.deepEqual(await claim(worker), { claimed: false, reason: "upload_backlog" });
  const first = uploading[0] ?? "";
  await addReadyVideo(first);
  assert.equal(await completeJob({ worker, jobId: first, attempt: 1, now: NOW }), "ok");
  assert.equal((await claim(worker)).claimed, true);
});

test("a running job whose worker keeps heartbeating is ended once it is far past its runtime limit", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a"), estimatedSeconds: 480 });
  await claimedId(worker);

  // maxRuntimeSeconds(480) is 1440 s, plus the 300 s margin.
  const edge = NOW + SHORT_LIMIT_MS;
  await recordHeartbeat({ worker, now: edge, body: heartbeat([entry(jobId)]) });
  assert.equal(await sweepLeases(edge), 0);

  await recordHeartbeat({ worker, now: edge + 1, body: heartbeat([entry(jobId)]) });
  assert.equal(await sweepLeases(edge + 1), 1);
  const job = await getJob(jobId);
  assert.equal(job.status, "queued");
  assert.equal(job.attempt, 1);
  const lost = await recordHeartbeat({ worker, now: edge + 2, body: heartbeat([entry(jobId)]) });
  assert.equal(lost.jobs[0]?.lease, "lost");
});

test("an upload that never ends is given up after six and a half hours, heartbeats or not", async () => {
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId: await makeUser("a") });
  await claimedId(worker);
  await toUploading(worker, jobId);
  const beat = (now: number) =>
    recordHeartbeat({ worker, now, body: heartbeat([entry(jobId, { phase: "uploading" })]) });

  await beat(NOW + UPLOAD_LIMIT_MS);
  assert.equal(await sweepLeases(NOW + UPLOAD_LIMIT_MS), 0);
  await beat(NOW + UPLOAD_LIMIT_MS + 1);
  assert.equal(await sweepLeases(NOW + UPLOAD_LIMIT_MS + 1), 1);
  assert.equal((await getJob(jobId)).status, "queued");
  assert.deepEqual((await eventTypes(jobId)).slice(-2), ["lease_expired", "requeued"]);
});

test("a cancel pending when the lease is lost on the last attempt ends canceled, not failed", async () => {
  const worker = await makeWorker();
  const userId = await makeUser("a");
  const jobId = await makeQueuedJob({ userId });
  await claimedId(worker);
  await sweepLeases(NOW + 2 * MINUTE);
  await claimedId(worker, NOW + 3 * MINUTE);
  await cancelJob({ jobId, by: "user", actorUserId: userId, now: NOW + 4 * MINUTE });

  await sweepLeases(NOW + 6 * MINUTE);
  const job = await getJob(jobId);
  assert.equal(job.status, "canceled");
  assert.equal(job.attempt, 2);
  assert.equal(job.failureCode, null);
  assert.equal(job.canceledBy, "user");
});

test("a job that fails three times with a machine fault is failed instead of blocking the queue again", async () => {
  const worker = await makeWorker();
  const poison = await makeQueuedJob({ userId: await makeUser("p"), enqueuedAt: NOW - 200 * MINUTE });
  const other = await makeQueuedJob({ userId: await makeUser("o"), enqueuedAt: NOW - 100 * MINUTE });
  const resume = () =>
    db.update(workers).set({ paused: false, pausedBy: null, pauseReason: null }).where(eq(workers.id, worker.id));

  const outcomes = [];
  for (let round = 0; round < 3; round += 1) {
    assert.equal(await claimedId(worker), poison);
    outcomes.push(
      await failJob({
        ...(await held(worker, poison)),
        now: NOW + round * MINUTE,
        body: failure("capture_incompatible", "AfxHookSource2"),
      }),
    );
    if (round < 2) await resume();
  }
  assert.deepEqual(outcomes, [
    { outcome: "requeued", workerPaused: true },
    { outcome: "requeued", workerPaused: true },
    { outcome: "failed", workerPaused: false },
  ]);
  const job = await getJob(poison);
  assert.equal(job.status, "failed");
  assert.equal(job.failureCode, "capture_incompatible");
  // Not the worker's sentence, which says the job is still in the queue.
  assert.match(job.failureReason ?? "", /varias veces/);
  assert.equal(job.machineRequeues, 2);
  assert.equal((await getWorker(worker.id)).paused, false);
  assert.equal(await claimedId(worker), other);

  // An operator retry starts the count again.
  await failJob({ ...(await held(worker, other)), now: NOW, body: failure("render_failed") });
  assert.equal(await retryJob({ jobId: poison, adminUserId: "admin-1", now: NOW }), "ok");
  assert.equal((await getJob(poison)).machineRequeues, 0);
});

test("a priority boost ends with the run it was given for, but survives an automatic requeue", async () => {
  const worker = await makeWorker();
  const boosted = await makeQueuedJob({ userId: await makeUser("a"), priorityBoost: 1 });
  const waiting = await makeQueuedJob({ userId: await makeUser("b"), enqueuedAt: NOW - 100 * MINUTE });

  assert.equal(await claimedId(worker), boosted);
  await failJob({ ...(await held(worker, boosted)), now: NOW, body: failure("capture_flake") });
  assert.equal((await getJob(boosted)).priorityBoost, 1);
  assert.equal(await claimedId(worker), boosted);

  await failJob({ ...(await held(worker, boosted)), now: NOW, body: failure("render_failed") });
  assert.equal((await getJob(boosted)).priorityBoost, 0);
  assert.equal(await retryJob({ jobId: boosted, adminUserId: "admin-1", now: NOW + 1000 }), "ok");
  assert.equal(await claimedId(worker, NOW + 2000), waiting);
});

test("a retry clears a boost the job still had when it was cancelled", async () => {
  const userId = await makeUser("a");
  const jobId = await makeQueuedJob({ userId, priorityBoost: 3 });
  await cancelJob({ jobId, by: "admin", actorUserId: "admin-1", now: NOW });
  assert.equal((await getJob(jobId)).priorityBoost, 0);

  await db.update(requests).set({ priorityBoost: 3 }).where(eq(requests.id, jobId));
  assert.equal(await retryJob({ jobId, adminUserId: "admin-1", now: NOW }), "ok");
  assert.equal((await getJob(jobId)).priorityBoost, 0);
});

test("a user cancelling a job that never ran frees its demo at once, unless another job needs it", async () => {
  const userId = await makeUser("a");
  const first = await makeQueuedJob({ userId });
  const second = await makeQueuedJob({ userId });
  const path = (await getJob(first)).demoPath ?? "";

  await cancelJob({ jobId: first, by: "user", actorUserId: userId, now: NOW });
  assert.equal(await demoFileSize(path), 18);
  assert.equal((await getJob(first)).demoPath, path);

  await cancelJob({ jobId: second, by: "user", actorUserId: userId, now: NOW });
  assert.equal(await demoFileSize(path), null);
  assert.equal((await getJob(second)).demoPath, null);
  assert.equal(await retryJob({ jobId: second, adminUserId: "admin-1", now: NOW }), "demo_gone");
});

test("the demo of a job an operator cancelled, or that already ran, is kept for a retry", async () => {
  const worker = await makeWorker();
  const userId = await makeUser("a");
  const byOperator = await makeQueuedJob({ userId });
  await cancelJob({ jobId: byOperator, by: "admin", actorUserId: "admin-1", now: NOW });
  const path = (await getJob(byOperator)).demoPath ?? "";
  assert.equal(await demoFileSize(path), 18);

  const other = await makeUser("b");
  const ran = await makeQueuedJob({ userId: other });
  await claimedId(worker);
  await failJob({ ...(await held(worker, ran)), now: NOW, body: failure("capture_flake") });
  await cancelJob({ jobId: ran, by: "user", actorUserId: other, now: NOW });
  assert.equal(await demoFileSize((await getJob(ran)).demoPath ?? ""), 18);
});
