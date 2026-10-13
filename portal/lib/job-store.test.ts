import {
  NOW,
  eventTypes,
  getJob,
  makeQueuedJob,
  makeUser,
  makeWorker,
  resetDatabase,
  shortSpec,
} from "./test-support.ts";

import { beforeEach, test } from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFile } from "node:fs/promises";

import { eq } from "drizzle-orm";

import { db } from "../db/client.ts";
import { requests, userCloud, workers } from "../db/schema.ts";
import { demoFileSize, demoPath } from "./demo-store.ts";
import {
  attachDemo,
  createJob,
  findUserJob,
  listUserJobs,
  userUsage,
  type CreateJobResult,
} from "./job-store.ts";
import { cancelJob, claimNext, failJob } from "./queue-store.ts";
import { storageStats } from "./storage-guard.ts";
import type { UserContext } from "./user-context.ts";
import { studioJobs, studioMe } from "./views.ts";

const DEMO = Buffer.concat([Buffer.from("PBDEMS2\0"), Buffer.alloc(4096, 7)]);
const DEMO_SHA256 = createHash("sha256").update(DEMO).digest("hex");

beforeEach(resetDatabase);

function session(userId: string): UserContext {
  return { userId, deviceId: null, via: "session" };
}

function body(overrides: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    kind: "short",
    title: "R3 4k - Kill Feed",
    demo: { sha256: DEMO_SHA256, sizeBytes: DEMO.length, fileName: "match.dem" },
    spec: shortSpec(),
    ...overrides,
  };
}

async function create(userId: string, overrides: Record<string, unknown> = {}) {
  return createJob({ user: session(userId), body: body(overrides), now: NOW });
}

async function createdId(userId: string): Promise<string> {
  const result = await create(userId);
  if (!result.ok) throw new Error(`job not created: ${result.code}`);
  return result.id;
}

function refusal(result: CreateJobResult): { status: number; code: string } | null {
  return result.ok ? null : { status: result.status, code: result.code };
}

function upload(bytes: Buffer): Request {
  return new Request("http://portal.test/api/studio/jobs/x/demo", {
    method: "PUT",
    body: new Uint8Array(bytes),
    headers: { "content-length": String(bytes.length) },
  });
}

test("a new job waits for its demo and carries the estimate the scheduler will use", async () => {
  const userId = await makeUser("luis");
  const result = await create(userId);
  assert.equal(result.ok, true);
  if (!result.ok) return;
  assert.equal(result.status, "awaiting_demo");
  assert.equal(result.demoUpload, "required");

  const job = await getJob(result.id);
  assert.equal(job.kind, "short");
  assert.equal(job.title, "R3 4k - Kill Feed");
  assert.equal(job.targetSteamId, "76561198000000001");
  // One 100 s window plus 2 s of padding: 240 + 1.3 * 102 + 1.5 * 102.
  assert.equal(job.estCaptureSeconds, 102);
  assert.equal(job.estimatedSeconds, 526);
  assert.equal(job.enqueuedAt, null);
  assert.deepEqual(await eventTypes(result.id), ["created"]);
});

test("uploading the demo queues the job; a second job from the same match needs no upload", async () => {
  const userId = await makeUser("luis");
  const first = await createdId(userId);

  const attached = await attachDemo({ user: session(userId), jobId: first, request: upload(DEMO) });
  assert.deepEqual(attached, { ok: true });
  const job = await getJob(first);
  assert.equal(job.status, "queued");
  assert.equal(job.demoSizeBytes, DEMO.length);
  assert.notEqual(job.enqueuedAt, null);
  assert.deepEqual(await readFile(job.demoPath ?? ""), DEMO);

  const second = await create(userId);
  assert.equal(second.ok, true);
  if (!second.ok) return;
  assert.equal(second.status, "queued");
  assert.equal(second.demoUpload, "present");
  assert.equal((await getJob(second.id)).demoPath, job.demoPath);
});

test("another user with the same demo hash still has to upload it", async () => {
  const luis = await makeUser("luis");
  await attachDemo({ user: session(luis), jobId: await createdId(luis), request: upload(DEMO) });

  const other = await create(await makeUser("other"));
  assert.equal(other.ok, true);
  if (other.ok) assert.equal(other.demoUpload, "required");
});

test("a demo that does not match the declared hash is rejected and the job keeps waiting", async () => {
  const userId = await makeUser("luis");
  const jobId = await createdId(userId);
  const tampered = Buffer.concat([DEMO, Buffer.from("x")]);

  const result = await attachDemo({ user: session(userId), jobId, request: upload(tampered) });
  assert.equal(result.ok, false);
  if (!result.ok) assert.deepEqual([result.status, result.code], [422, "sha256_mismatch"]);
  const job = await getJob(jobId);
  assert.equal(job.status, "awaiting_demo");
  assert.equal(job.demoPath, null);

  assert.deepEqual(await attachDemo({ user: session(userId), jobId, request: upload(DEMO) }), { ok: true });
});

test("a CS:GO demo and a file that is no demo get different answers", async () => {
  const userId = await makeUser("luis");
  const jobId = await createdId(userId);
  const attach = (bytes: Buffer) => attachDemo({ user: session(userId), jobId, request: upload(bytes) });

  const csgo = await attach(Buffer.from("HL2DEMO\0old source 1 demo"));
  const text = await attach(Buffer.from("just some text, not a demo"));
  assert.deepEqual([csgo.ok ? 0 : csgo.status, csgo.ok ? "" : csgo.code], [415, "csgo_demo"]);
  assert.deepEqual([text.ok ? 0 : text.status, text.ok ? "" : text.code], [415, "not_a_demo"]);
});

test("a demo over the size cap is refused before and during the upload", async () => {
  const userId = await makeUser("luis");
  const declared = await create(userId, {
    demo: { sha256: DEMO_SHA256, sizeBytes: 700 * 1024 * 1024 + 1, fileName: "big.dem" },
  });
  assert.deepEqual(refusal(declared), { status: 413, code: "demo_too_large" });

  const jobId = await createdId(userId);
  process.env.MAX_DEMO_BYTES = "1024";
  try {
    const result = await attachDemo({ user: session(userId), jobId, request: upload(DEMO) });
    assert.deepEqual([result.ok ? 0 : result.status, result.ok ? "" : result.code], [413, "demo_too_large"]);
  } finally {
    delete process.env.MAX_DEMO_BYTES;
  }
  assert.equal((await getJob(jobId)).status, "awaiting_demo");
});

test("nobody can upload a demo into, read or list another user's job", async () => {
  const owner = await makeUser("owner");
  const stranger = await makeUser("stranger");
  const jobId = await createdId(owner);

  const result = await attachDemo({ user: session(stranger), jobId, request: upload(DEMO) });
  assert.deepEqual([result.ok ? 0 : result.status, result.ok ? "" : result.code], [404, "not_found"]);
  assert.equal(await findUserJob(stranger, jobId), null);
  assert.equal((await findUserJob(owner, jobId))?.id, jobId);
  assert.deepEqual(await listUserJobs(stranger, NOW), []);
  assert.equal((await listUserJobs(owner, NOW)).length, 1);
});

test("an upload for a job that is no longer waiting is refused", async () => {
  const userId = await makeUser("luis");
  const jobId = await createdId(userId);
  await db.update(requests).set({ status: "canceled" }).where(eq(requests.id, jobId));

  const result = await attachDemo({ user: session(userId), jobId, request: upload(DEMO) });
  assert.deepEqual([result.ok ? 0 : result.status, result.ok ? "" : result.code], [409, "invalid_state"]);
});

test("a user without access cannot create jobs", async () => {
  const userId = await makeUser("luis");
  await db.insert(userCloud).values({ userId, access: "pending", updatedAt: NOW });
  assert.deepEqual(refusal(await create(userId)), { status: 403, code: "cloud_access_pending" });

  await db.update(userCloud).set({ access: "blocked" }).where(eq(userCloud.userId, userId));
  assert.deepEqual(refusal(await create(userId)), { status: 403, code: "cloud_access_blocked" });
});

test("a full demo is refused while the kind is switched off", async () => {
  const result = await create(await makeUser("luis"), { kind: "full_demo" });
  assert.deepEqual(refusal(result), { status: 503, code: "kind_unavailable" });
});

test("an invalid spec is refused with the reason", async () => {
  const userId = await makeUser("luis");
  const result = await create(userId, { spec: { ...shortSpec(), targetSteamId: "nope" } });
  assert.deepEqual(refusal(result), { status: 422, code: "invalid_spec" });
  if (!result.ok) assert.match(result.error, /targetSteamId/);
  assert.deepEqual(refusal(await create(userId, { kind: "long" })), { status: 422, code: "invalid_spec" });
});

test("the fourth active job of a user is refused", async () => {
  const userId = await makeUser("luis");
  for (let index = 0; index < 3; index += 1) await createdId(userId);
  assert.deepEqual(refusal(await create(userId)), { status: 429, code: "limit_active" });
  assert.equal((await create(await makeUser("other"))).ok, true);
});

test("the daily budget counts used time, waiting jobs and the new job", async () => {
  const userId = await makeUser("luis");
  // 5400 s a day by default; each job here is estimated at 526 s.
  await db.insert(requests).values({
    userId,
    kind: "short",
    status: "done",
    finishedAt: NOW - 60_000,
    machineSeconds: 4400,
    estimatedSeconds: 526,
  });
  assert.equal((await create(userId)).ok, true);
  assert.deepEqual(refusal(await create(userId)), { status: 429, code: "limit_daily" });

  await db.insert(userCloud).values({ userId, access: "allowed", dailySeconds: 20000, updatedAt: NOW });
  assert.equal((await create(userId)).ok, true);
});

test("the cloud refuses new jobs when the queue is full or the volume is nearly full", async () => {
  const userId = await makeUser("luis");
  await makeQueuedJob({ userId: await makeUser("other") });
  process.env.MAX_QUEUED_JOBS = "1";
  try {
    assert.deepEqual(refusal(await create(userId)), { status: 503, code: "cloud_queue_full" });
  } finally {
    delete process.env.MAX_QUEUED_JOBS;
  }
  process.env.MIN_FREE_BYTES = String(Number.MAX_SAFE_INTEGER);
  try {
    assert.deepEqual(refusal(await create(userId)), { status: 503, code: "cloud_storage_full" });
  } finally {
    process.env.MIN_FREE_BYTES = "1";
  }
});

test("usage separates what is running from what is still waiting", async () => {
  const userId = await makeUser("luis");
  const worker = await makeWorker();
  await makeQueuedJob({ userId, estimatedSeconds: 480, enqueuedAt: NOW - 1000 });
  await makeQueuedJob({ userId, estimatedSeconds: 300 });
  await claimNext({ worker, kinds: ["short"], now: NOW });

  assert.deepEqual(await userUsage(db, { userId, now: NOW }), {
    active: 2,
    activeFullDemo: 0,
    secondsLast24h: 480,
    secondsCommitted: 300,
  });
});

test("the job view shows the queue position only while queued and the failure only when failed", async () => {
  const userId = await makeUser("luis");
  await makeWorker();
  const queued = await makeQueuedJob({ userId, enqueuedAt: NOW - 1000 });
  const failed = await makeQueuedJob({ userId });
  await db
    .update(requests)
    .set({ status: "failed", failureCode: "render_failed", failureReason: "El render falló.", failureDetail: "ffmpeg exit 1" })
    .where(eq(requests.id, failed));

  const views = await studioJobs(await listUserJobs(userId, NOW), NOW);
  const queuedView = views.find((view) => view.id === queued);
  const failedView = views.find((view) => view.id === failed);
  assert.deepEqual(queuedView?.queue, {
    position: 1,
    estimatedStartAt: NOW,
    estimatedDoneAt: NOW + 480_000,
    state: "online",
  });
  assert.equal(queuedView?.failure, null);
  assert.equal(failedView?.queue, null);
  assert.deepEqual(failedView?.failure, { code: "render_failed", message: "El render falló." });
  // The raw cause is for the operator; it must never reach the user's view.
  assert.equal(JSON.stringify(views).includes("ffmpeg exit 1"), false);
});

test("me reports access, limits, usage and the wait a new job would have", async () => {
  const userId = await makeUser("luis");
  assert.equal((await studioMe(userId, NOW))?.queue.waitSeconds.short, null);

  const worker = await makeWorker();
  await makeQueuedJob({ userId: await makeUser("other"), estimatedSeconds: 600 });
  await claimNext({ worker, kinds: ["short"], now: NOW });
  await makeQueuedJob({ userId: await makeUser("third"), estimatedSeconds: 300, enqueuedAt: NOW - 60_000 });

  const me = await studioMe(userId, NOW + 6000);
  assert.equal(me?.access, "allowed");
  assert.deepEqual(me?.limits, { maxActive: 3, dailySeconds: 5400, maxDemoBytes: 734003200 });
  assert.deepEqual(me?.usage, { active: 0, secondsLast24h: 0, secondsCommitted: 0 });
  assert.deepEqual(me?.kinds, ["short"]);
  assert.equal(me?.queue.state, "online");
  assert.equal(me?.queue.queued, 1);
  // 594 s left of the running job, then the 300 s job that was already waiting.
  assert.equal(me?.queue.waitSeconds.short, 894);
});

test("a demo file is only on disk once per user and hash", async () => {
  const userId = await makeUser("luis");
  const first = await createdId(userId);
  const second = await createdId(userId);
  await attachDemo({ user: session(userId), jobId: first, request: upload(DEMO) });
  await attachDemo({ user: session(userId), jobId: second, request: upload(DEMO) });

  const [a, b] = [await getJob(first), await getJob(second)];
  assert.equal(a.demoPath, b.demoPath);
  assert.equal(await demoFileSize(a.demoPath ?? ""), DEMO.length);
});

const MIB = 1024 * 1024;

function demoBytes(fill: number): Buffer {
  return Buffer.concat([Buffer.from("PBDEMS2\0"), Buffer.alloc(4096, fill)]);
}

function demoOf(bytes: Buffer, sizeBytes = bytes.length): Record<string, unknown> {
  return {
    demo: {
      sha256: createHash("sha256").update(bytes).digest("hex"),
      sizeBytes,
      fileName: "match.dem",
    },
  };
}

interface HeldUpload {
  request: Request;
  // Sends the bytes and ends the body.
  finish: () => void;
}

// An upload whose body stays open, like a slow client, until finish is called.
function heldUpload(bytes: Buffer, declared = bytes.length): HeldUpload {
  let finish = () => {};
  const body = new ReadableStream<Uint8Array>({
    start(controller) {
      finish = () => {
        controller.enqueue(new Uint8Array(bytes));
        controller.close();
      };
    },
  });
  // Node needs duplex for a streamed body; the DOM type of RequestInit does not know it.
  const init: RequestInit & { duplex: "half" } = {
    method: "PUT",
    body,
    headers: { "content-length": String(declared) },
    duplex: "half",
  };
  return { request: new Request("http://portal.test/api/studio/jobs/x/demo", init), finish };
}

// The same, already sent in full.
function sentUpload(bytes: Buffer, declared: number): Request {
  const held = heldUpload(bytes, declared);
  held.finish();
  return held.request;
}

function code(result: { ok: true } | { ok: false; status: number; code: string }) {
  return result.ok ? [200, "ok"] : [result.status, result.code];
}

// Lets the uploads started so far reach their body before the next call is made.
async function settle(): Promise<void> {
  await new Promise((resolve) => setTimeout(resolve, 50));
}

test("a job takes one demo upload at a time: a parallel one is told to wait", async () => {
  const userId = await makeUser("luis");
  const jobId = await createdId(userId);
  const slow = heldUpload(DEMO);
  const first = attachDemo({ user: session(userId), jobId, request: slow.request });
  await settle();

  const second = await attachDemo({ user: session(userId), jobId, request: upload(DEMO) });
  assert.deepEqual(code(second), [409, "upload_in_progress"]);

  slow.finish();
  assert.deepEqual(await first, { ok: true });
  assert.equal((await getJob(jobId)).status, "queued");
});

test("a user uploads two demos at once at most; the slot is free again when one ends or fails", async () => {
  const userId = await makeUser("luis");
  const jobs = [await createdId(userId), await createdId(userId), await createdId(userId)];
  const [a, b] = [heldUpload(DEMO), heldUpload(Buffer.from("not a demo at all"))];
  const first = attachDemo({ user: session(userId), jobId: jobs[0] ?? "", request: a.request });
  const second = attachDemo({ user: session(userId), jobId: jobs[1] ?? "", request: b.request });
  await settle();

  const third = () => attachDemo({ user: session(userId), jobId: jobs[2] ?? "", request: upload(DEMO) });
  assert.deepEqual(code(await third()), [429, "limit_uploads"]);

  const other = await makeUser("other");
  const theirs = await attachDemo({ user: session(other), jobId: await createdId(other), request: upload(DEMO) });
  assert.deepEqual(theirs, { ok: true });

  b.finish();
  assert.deepEqual(code(await second), [415, "not_a_demo"]);
  assert.deepEqual(await third(), { ok: true });
  a.finish();
  assert.deepEqual(await first, { ok: true });
});

test("uploads in flight reserve their declared size against the free space floor", async () => {
  const { freeBytes } = await storageStats();
  // Sizes far above what other processes write to the volume while the test runs.
  const unit = Math.min(200 * MIB, Math.floor(freeBytes / 8));
  const luis = await makeUser("luis");
  const other = await makeUser("other");
  const mine = await createdId(luis);
  const theirs = await createdId(other);
  const attach = (userId: string, jobId: string, request: Request) =>
    attachDemo({ user: session(userId), jobId, request });

  process.env.MIN_FREE_BYTES = String(freeBytes - 3 * unit);
  try {
    // Alone, a 2 unit upload leaves the volume above the floor: it gets as far as its content.
    const alone = await attach(other, theirs, sentUpload(Buffer.from("not a demo at all"), 2 * unit));
    assert.deepEqual(code(alone), [415, "not_a_demo"]);

    const slow = heldUpload(DEMO, 2 * unit);
    const first = attach(luis, mine, slow.request);
    await settle();
    const refused = await attach(other, theirs, heldUpload(DEMO, 2 * unit).request);
    assert.deepEqual(code(refused), [503, "cloud_storage_full"]);

    slow.finish();
    assert.deepEqual(await first, { ok: true });
  } finally {
    process.env.MIN_FREE_BYTES = "1";
  }
});

test("a body longer than its declared length is cut off, because only that much was reserved", async () => {
  const userId = await makeUser("luis");
  const jobId = await createdId(userId);
  const result = await attachDemo({
    user: session(userId),
    jobId,
    request: sentUpload(DEMO, 100),
  });
  assert.deepEqual(code(result), [413, "demo_too_large"]);
  assert.equal((await getJob(jobId)).status, "awaiting_demo");
});

test("a user cannot hold more demo bytes than the quota: refused when creating and when uploading", async () => {
  const userId = await makeUser("luis");
  const [a, b, c] = [demoBytes(1), demoBytes(2), demoBytes(3)];
  process.env.MAX_DEMO_BYTES_PER_USER = String(2 * a.length + 10);
  process.env.MAX_ACTIVE_REQUESTS_PER_USER = "10";
  try {
    const first = await create(userId, demoOf(a));
    assert.equal(first.ok, true);
    // The same match again adds nothing; a second match fits; a third does not.
    assert.equal((await create(userId, demoOf(a))).ok, true);
    const second = await create(userId, demoOf(b));
    assert.equal(second.ok, true);
    assert.deepEqual(refusal(await create(userId, demoOf(c))), { status: 429, code: "limit_storage" });
    assert.equal((await create(await makeUser("other"), demoOf(c))).ok, true);

    // Declaring a small size when creating does not get a large demo past the quota.
    const understated = await create(userId, demoOf(c, 5));
    assert.equal(understated.ok, true);
    if (!understated.ok) return;
    const sent = await attachDemo({ user: session(userId), jobId: understated.id, request: upload(c) });
    assert.deepEqual(code(sent), [429, "limit_storage"]);
    assert.equal((await getJob(understated.id)).status, "awaiting_demo");

    // Cancelling the job that waited for the second match gives the room back.
    if (second.ok) await cancelJob({ jobId: second.id, by: "user", actorUserId: userId, now: NOW });
    const retried = await attachDemo({ user: session(userId), jobId: understated.id, request: upload(c) });
    assert.deepEqual(retried, { ok: true });
  } finally {
    delete process.env.MAX_DEMO_BYTES_PER_USER;
    delete process.env.MAX_ACTIVE_REQUESTS_PER_USER;
  }
});

async function reportPlanSchema(workerId: string, planSchema: string): Promise<void> {
  await db.update(workers).set({ health: JSON.stringify({ planSchema }) }).where(eq(workers.id, workerId));
}

function specFrom(planSchema: string | null): Record<string, unknown> {
  const client = planSchema === null ? { studioVersion: "5.4.4" } : { studioVersion: "5.4.4", planSchema };
  return { spec: { ...shortSpec(), client } };
}

test("a Studio whose plan schema differs from the worker's is refused before anything is created", async () => {
  const userId = await makeUser("luis");
  const worker = await makeWorker();
  await reportPlanSchema(worker.id, "kill-plan/3");

  const result = await create(userId, specFrom("kill-plan/2"));
  assert.deepEqual(refusal(result), { status: 409, code: "studio_version_mismatch" });
  assert.deepEqual(await listUserJobs(userId, NOW), []);

  assert.equal((await create(userId, specFrom("kill-plan/3"))).ok, true);
  // A Studio that does not say, and a worker that does not say, are let through.
  assert.equal((await create(userId, specFrom(null))).ok, true);
  await reportPlanSchema(worker.id, "");
  assert.equal((await create(userId, specFrom("kill-plan/2"))).ok, true);
});

test("the version gate follows the worker seen last and ignores revoked ones", async () => {
  const userId = await makeUser("luis");
  const old = await makeWorker("old");
  const current = await makeWorker("current");
  await reportPlanSchema(old.id, "kill-plan/2");
  await reportPlanSchema(current.id, "kill-plan/3");
  await db.update(workers).set({ lastSeenAt: NOW - 60_000 }).where(eq(workers.id, old.id));

  assert.deepEqual(refusal(await create(userId, specFrom("kill-plan/2"))), {
    status: 409,
    code: "studio_version_mismatch",
  });
  await db.update(workers).set({ revokedAt: NOW }).where(eq(workers.id, current.id));
  assert.equal((await create(userId, specFrom("kill-plan/2"))).ok, true);
});

test("a job that runs again after a quick failure still counts its whole estimate against the day", async () => {
  const userId = await makeUser("luis");
  const worker = await makeWorker();
  const jobId = await makeQueuedJob({ userId, estimatedSeconds: 2700 });
  await db.insert(userCloud).values({ userId, access: "allowed", dailySeconds: 3000, updatedAt: NOW });

  await claimNext({ worker, kinds: ["short"], now: NOW });
  assert.equal((await userUsage(db, { userId, now: NOW })).secondsLast24h, 2700);
  await failJob({
    worker,
    jobId,
    attempt: 1,
    now: NOW + 5000,
    body: { code: "demo_download_failed", message: "m", detail: "d", machineSeconds: 5 },
  });
  await claimNext({ worker, kinds: ["short"], now: NOW + 6000 });

  assert.equal((await userUsage(db, { userId, now: NOW + 6000 })).secondsLast24h, 2705);
  // 2705 s in use plus a 526 s job is over the 3000 s this user has.
  assert.deepEqual(refusal(await createJob({ user: session(userId), body: body(), now: NOW + 7000 })), {
    status: 429,
    code: "limit_daily",
  });
});

test("a demo that finishes uploading after its job was cancelled is not left on the volume", async () => {
  const userId = await makeUser("luis");
  const jobId = await createdId(userId);
  const slow = heldUpload(DEMO);
  const pending = attachDemo({ user: session(userId), jobId, request: slow.request });
  await settle();
  await cancelJob({ jobId, by: "user", actorUserId: userId, now: NOW });

  slow.finish();
  assert.deepEqual(code(await pending), [409, "invalid_state"]);
  assert.equal(await demoFileSize(demoPath(userId, DEMO_SHA256)), null);
});
