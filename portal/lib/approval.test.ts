import {
  NOW,
  eventTypes,
  getJob,
  makeUser,
  makeWorker,
  resetDatabase,
  shortSpec,
} from "./test-support.ts";

import { afterEach, beforeEach, test } from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";

import { updateUserCloud } from "./admin-store.ts";
import { demoFileSize } from "./demo-store.ts";
import { attachDemo, createJob } from "./job-store.ts";
import { approveJob, cancelJob, claimNext, rejectJob, retryJob } from "./queue-store.ts";
import type { UserContext } from "./user-context.ts";
import { loadApprovalMode, needsApproval } from "./user-limits.ts";
import { adminOverview, studioJobs, studioMe } from "./views.ts";

const DEMO = Buffer.concat([Buffer.from("PBDEMS2\0"), Buffer.alloc(4096, 7)]);
const DEMO_SHA256 = createHash("sha256").update(DEMO).digest("hex");
const ADMIN = "operator";

beforeEach(async () => {
  await resetDatabase();
  process.env.CLOUD_APPROVAL = "required";
});

afterEach(() => {
  process.env.CLOUD_APPROVAL = "off";
});

function session(userId: string): UserContext {
  return { userId, deviceId: null, via: "session" };
}

// Creates a Short and uploads its demo, which is when the approval gate applies.
async function submit(userId: string): Promise<string> {
  const created = await createJob({
    user: session(userId),
    body: {
      kind: "short",
      title: "R3 4k",
      demo: { sha256: DEMO_SHA256, sizeBytes: DEMO.length, fileName: "match.dem" },
      spec: shortSpec(),
    },
    now: NOW,
  });
  if (!created.ok) throw new Error(`job not created: ${created.code}`);
  if (created.demoUpload === "present") return created.id;
  const request = new Request("http://portal.test/api/studio/jobs/x/demo", {
    method: "PUT",
    body: new Uint8Array(DEMO),
    headers: { "content-length": String(DEMO.length) },
  });
  const attached = await attachDemo({ user: session(userId), jobId: created.id, request });
  if (!attached.ok) throw new Error(`demo not attached: ${attached.code}`);
  return created.id;
}

async function claim() {
  return claimNext({ worker: await makeWorker(), kinds: ["short"], now: NOW });
}

test("approval is required unless the portal is told otherwise", () => {
  assert.equal(loadApprovalMode({}), "required");
  assert.equal(loadApprovalMode({ CLOUD_APPROVAL: "off" }), "off");
  assert.equal(loadApprovalMode({ CLOUD_APPROVAL: "sometimes" }), "required");
});

test("only an untrusted user's job waits for the operator", () => {
  const base = { mode: "required", isAdmin: false, autoApprove: false } as const;
  assert.equal(needsApproval(base), true);
  assert.equal(needsApproval({ ...base, isAdmin: true }), false);
  assert.equal(needsApproval({ ...base, autoApprove: true }), false);
  assert.equal(needsApproval({ ...base, mode: "off" }), false);
});

test("a submitted job waits for approval and no worker can take it", async () => {
  const userId = await makeUser("luis");
  const jobId = await submit(userId);

  const job = await getJob(jobId);
  assert.equal(job.status, "pending_approval");
  assert.equal(job.enqueuedAt, null);
  assert.deepEqual(await eventTypes(jobId), ["created", "demo_uploaded"]);
  assert.deepEqual(await claim(), { claimed: false, reason: "empty" });

  const overview = await adminOverview(NOW);
  assert.deepEqual(overview.requests.map((request) => request.id), [jobId]);
  assert.equal(overview.queue.length, 0);
});

test("approving a request puts it in the queue, where a worker takes it", async () => {
  const userId = await makeUser("luis");
  const jobId = await submit(userId);

  assert.equal(await approveJob({ jobId, adminUserId: ADMIN, now: NOW + 5000 }), "ok");
  const job = await getJob(jobId);
  assert.equal(job.status, "queued");
  // Seniority in the queue counts from the approval, not from the request.
  assert.equal(job.enqueuedAt, NOW + 5000);
  assert.deepEqual((await eventTypes(jobId)).sort(), ["approved", "created", "demo_uploaded", "queued"]);
  assert.equal(await approveJob({ jobId, adminUserId: ADMIN, now: NOW + 6000 }), "invalid_state");

  const claimed = await claimNext({ worker: await makeWorker(), kinds: ["short"], now: NOW + 7000 });
  assert.equal(claimed.claimed, true);
  if (claimed.claimed) assert.equal(claimed.job.id, jobId);
});

test("a rejected request ends with the operator's reason and keeps its demo for a retry", async () => {
  const userId = await makeUser("luis");
  const jobId = await submit(userId);

  const rejected = await rejectJob({ jobId, reason: "  La demo no es tuya  ", adminUserId: ADMIN, now: NOW });
  assert.equal(rejected, "ok");
  const job = await getJob(jobId);
  assert.equal(job.status, "canceled");
  assert.equal(job.canceledBy, "admin");
  assert.notEqual(await demoFileSize(job.demoPath ?? ""), null);

  const [view] = await studioJobs([job], NOW);
  assert.deepEqual(view?.failure, { code: "rejected", message: "La demo no es tuya" });
  assert.equal(await rejectJob({ jobId, reason: "", adminUserId: ADMIN, now: NOW }), "invalid_state");

  // The operator changing their mind is a retry, which needs no second approval.
  assert.equal(await retryJob({ jobId, adminUserId: ADMIN, now: NOW + 1000 }), "ok");
  const retried = await getJob(jobId);
  assert.equal(retried.status, "queued");
  const [again] = await studioJobs([retried], NOW + 1000);
  assert.equal(again?.failure, null);
});

test("a trusted user's job goes straight to the queue", async () => {
  const userId = await makeUser("luis");
  await updateUserCloud({ userId, patch: { autoApprove: true }, adminUserId: ADMIN, now: NOW });

  const job = await getJob(await submit(userId));
  assert.equal(job.status, "queued");
  assert.notEqual(job.enqueuedAt, null);
});

test("Studio is told whether a new job of this user waits for approval", async () => {
  const userId = await makeUser("luis");
  assert.equal((await studioMe(userId, NOW))?.needsApproval, true);

  await updateUserCloud({ userId, patch: { autoApprove: true }, adminUserId: ADMIN, now: NOW });
  assert.equal((await studioMe(userId, NOW))?.needsApproval, false);
});

test("a user can withdraw a request, and it can no longer be approved", async () => {
  const userId = await makeUser("luis");
  const jobId = await submit(userId);

  const canceled = await cancelJob({ jobId, by: "user", actorUserId: userId, now: NOW });
  assert.deepEqual(canceled, { status: "canceled", cancelRequested: false });
  assert.equal(await approveJob({ jobId, adminUserId: ADMIN, now: NOW }), "invalid_state");
});

test("requests waiting for approval count against the user's active jobs", async () => {
  const userId = await makeUser("luis");
  await updateUserCloud({ userId, patch: { maxActive: 1 }, adminUserId: ADMIN, now: NOW });
  await submit(userId);

  const second = await createJob({
    user: session(userId),
    body: {
      kind: "short",
      demo: { sha256: DEMO_SHA256, sizeBytes: DEMO.length, fileName: "match.dem" },
      spec: shortSpec(),
    },
    now: NOW,
  });
  assert.equal(second.ok, false);
  if (!second.ok) assert.equal(second.code, "limit_active");
});
