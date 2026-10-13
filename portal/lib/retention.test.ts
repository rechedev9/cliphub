import {
  DEMO_SHA,
  MINUTE,
  NOW,
  getJob,
  makeQueuedJob,
  makeUser,
  resetDatabase,
} from "./test-support.ts";
import { TEST_DIR } from "./test-env.ts";

import { beforeEach, test } from "node:test";
import assert from "node:assert/strict";
import { mkdir, stat, utimes, writeFile } from "node:fs/promises";
import { dirname, join } from "node:path";

import { eq } from "drizzle-orm";

import { db } from "../db/client.ts";
import { deviceLinks, events, requestArtifacts, requests } from "../db/schema.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import { beginDemoUpload, demoTempPath } from "./demo-store.ts";
import { recordEvent } from "./events.ts";
import {
  loadRetentionConfig,
  retentionCutoffs,
  runRetentionSweep,
  type RetentionConfig,
} from "./retention.ts";

const HOUR = 60 * MINUTE;
const DAY = 24 * HOUR;
const CONFIG: RetentionConfig = {
  retentionDays: 30,
  artifactRetentionDays: 7,
  abandonedUploadHours: 24,
  sweepIntervalMs: 1000,
};

beforeEach(resetDatabase);

async function exists(path: string): Promise<boolean> {
  return (await stat(path).catch(() => null)) !== null;
}

async function sweep() {
  return runRetentionSweep(new Date(NOW), CONFIG);
}

interface ArtifactOptions {
  jobId: string;
  name: string;
  uploadedAt: number;
  receivedAt?: number;
  status?: string;
}

async function addArtifact(options: ArtifactOptions): Promise<string> {
  const path = join(TEST_DIR, "uploads", options.jobId, "artifacts", `${options.name}.bin`);
  await mkdir(dirname(path), { recursive: true });
  await writeFile(path, "video-bytes");
  await db.insert(requestArtifacts).values({
    requestId: options.jobId,
    variant: "viral-60-clean",
    name: options.name,
    path,
    sizeBytes: 11,
    sha256: DEMO_SHA,
    status: options.status ?? "ready",
    uploadedAt: new Date(options.uploadedAt),
    receivedAt: options.receivedAt ?? null,
  });
  return path;
}

async function finish(jobId: string, options: { status: string; finishedAt: number }) {
  await db.update(requests).set(options).where(eq(requests.id, jobId));
}

test("loadRetentionConfig falls back to sane defaults", () => {
  assert.deepEqual(loadRetentionConfig({}), {
    retentionDays: 30,
    artifactRetentionDays: 7,
    abandonedUploadHours: 24,
    sweepIntervalMs: 10 * MINUTE,
  });
});

test("loadRetentionConfig reads env overrides", () => {
  assert.deepEqual(
    loadRetentionConfig({
      RETENTION_DAYS: "14",
      ARTIFACT_RETENTION_DAYS: "3",
      ABANDONED_UPLOAD_HOURS: "6",
      RETENTION_SWEEP_INTERVAL_MS: "1000",
    }),
    { retentionDays: 14, artifactRetentionDays: 3, abandonedUploadHours: 6, sweepIntervalMs: 1000 },
  );
});

test("retentionCutoffs subtracts the configured windows from now", () => {
  const now = new Date("2026-09-07T12:00:00.000Z");
  const cutoffs = retentionCutoffs(now, CONFIG);
  assert.equal(cutoffs.terminalBefore.toISOString(), "2026-08-08T12:00:00.000Z");
  assert.equal(cutoffs.awaitingDemoBefore.toISOString(), "2026-09-06T12:00:00.000Z");
  assert.equal(new Date(cutoffs.linksBefore).toISOString(), "2026-09-07T11:00:00.000Z");
  assert.equal(new Date(cutoffs.eventsBefore).toISOString(), "2026-06-09T12:00:00.000Z");
});

test("a result Studio confirmed is deleted 24 hours later, an unconfirmed one after 7 days", async () => {
  const jobId = await makeQueuedJob({ userId: await makeUser("luis") });
  await finish(jobId, { status: "done", finishedAt: NOW - 8 * DAY });
  const confirmed = await addArtifact({
    jobId,
    name: "confirmed",
    uploadedAt: NOW - 2 * DAY,
    receivedAt: NOW - DAY - 1,
  });
  const justConfirmed = await addArtifact({
    jobId,
    name: "just-confirmed",
    uploadedAt: NOW - 2 * DAY,
    receivedAt: NOW - HOUR,
  });
  const old = await addArtifact({ jobId, name: "old", uploadedAt: NOW - 7 * DAY - 1 });
  const recent = await addArtifact({ jobId, name: "recent", uploadedAt: NOW - 6 * DAY });

  const result = await sweep();
  assert.equal(result.expiredArtifacts, 2);
  assert.deepEqual(
    [await exists(confirmed), await exists(justConfirmed), await exists(old), await exists(recent)],
    [false, true, false, true],
  );
  const left = await db.select({ name: requestArtifacts.name }).from(requestArtifacts);
  assert.deepEqual(left.map((row) => row.name).sort(), ["just-confirmed", "recent"]);
  assert.equal((await getJob(jobId)).status, "done");
});

test("a half-uploaded file is kept while its job is active and removed once the job is over", async () => {
  const active = await makeQueuedJob({ userId: await makeUser("a") });
  await db.update(requests).set({ status: "uploading" }).where(eq(requests.id, active));
  const inFlight = await addArtifact({ jobId: active, name: "part", uploadedAt: NOW - 30 * DAY, status: "uploading" });

  const over = await makeQueuedJob({ userId: await makeUser("b") });
  await finish(over, { status: "failed", finishedAt: NOW - HOUR });
  const stale = await addArtifact({ jobId: over, name: "part", uploadedAt: NOW - HOUR, status: "uploading" });

  await sweep();
  assert.deepEqual([await exists(inFlight), await exists(stale)], [true, false]);
});

test("a demo goes when its job is done, but stays 48 hours after a failure so it can be retried", async () => {
  const done = await makeQueuedJob({ userId: await makeUser("done") });
  await finish(done, { status: "done", finishedAt: NOW - MINUTE });
  const failed = await makeQueuedJob({ userId: await makeUser("failed") });
  await finish(failed, { status: "failed", finishedAt: NOW - 47 * HOUR });
  const longFailed = await makeQueuedJob({ userId: await makeUser("long-failed") });
  await finish(longFailed, { status: "canceled", finishedAt: NOW - 49 * HOUR });
  const queued = await makeQueuedJob({ userId: await makeUser("queued") });

  const paths = new Map<string, string>();
  for (const id of [done, failed, longFailed, queued]) {
    paths.set(id, (await getJob(id)).demoPath ?? "");
  }
  const result = await sweep();

  assert.equal(result.removedDemos, 2);
  assert.equal(await exists(paths.get(done) ?? ""), false);
  assert.equal(await exists(paths.get(failed) ?? ""), true);
  assert.equal(await exists(paths.get(longFailed) ?? ""), false);
  assert.equal(await exists(paths.get(queued) ?? ""), true);
  assert.equal((await getJob(done)).demoPath, null);
  assert.notEqual((await getJob(failed)).demoPath, null);
});

test("a demo shared by a finished job and a waiting one is kept", async () => {
  const userId = await makeUser("luis");
  const done = await makeQueuedJob({ userId });
  await finish(done, { status: "done", finishedAt: NOW - MINUTE });
  const queued = await makeQueuedJob({ userId });
  const path = (await getJob(queued)).demoPath ?? "";

  const result = await sweep();
  assert.equal(result.removedDemos, 0);
  assert.equal(await exists(path), true);
  assert.equal((await getJob(done)).demoPath, path);
});

test("an upload left behind by a crash is removed after an hour, a running one is not", async () => {
  const userId = await makeUser("luis");
  const abandoned = demoTempPath(userId, "abandoned");
  const running = demoTempPath(userId, "running");
  await mkdir(dirname(abandoned), { recursive: true });
  await writeFile(abandoned, "partial");
  await writeFile(running, "partial");
  await utimes(abandoned, new Date(NOW - 2 * HOUR), new Date(NOW - 2 * HOUR));
  await utimes(running, new Date(NOW - MINUTE), new Date(NOW - MINUTE));

  await sweep();
  assert.deepEqual([await exists(abandoned), await exists(running)], [false, true]);
});

test("a job whose demo never arrived is dropped after 24 hours", async () => {
  const userId = await makeUser("luis");
  const [stale] = await db
    .insert(requests)
    .values({ userId, kind: "short", status: "awaiting_demo", createdAt: new Date(NOW - 25 * HOUR) })
    .returning({ id: requests.id });
  const [fresh] = await db
    .insert(requests)
    .values({ userId, kind: "short", status: "awaiting_demo", createdAt: new Date(NOW - 23 * HOUR) })
    .returning({ id: requests.id });

  const result = await sweep();
  assert.equal(result.reapedAbandoned, 1);
  const left = await db.select({ id: requests.id }).from(requests);
  assert.deepEqual(left.map((row) => row.id), [fresh?.id]);
  assert.notEqual(stale?.id, fresh?.id);
});

test("videos of the old manual flow still expire after the retention window", async () => {
  const userId = await makeUser("luis");
  const [legacy] = await db
    .insert(requests)
    .values({ userId, status: "done", updatedAt: new Date(NOW - 31 * DAY) })
    .returning({ id: requests.id });
  if (!legacy) throw new Error("legacy row not created");
  const path = await addArtifact({ jobId: legacy.id, name: "reel", uploadedAt: NOW - 31 * DAY });
  await db.update(requests).set({ finalVideoPath: path }).where(eq(requests.id, legacy.id));

  const result = await sweep();
  assert.equal(result.expiredDeliveries, 1);
  assert.equal(await exists(path), false);
  const row = await getJob(legacy.id);
  assert.equal(row.status, "done");
  assert.equal(row.finalVideoPath, null);
});

test("expired link codes and events older than 90 days are pruned", async () => {
  const link = { pollTokenHash: "h", deviceName: "pc", createdAt: NOW };
  await db.insert(deviceLinks).values([
    { ...link, userCode: "AAAAAAAA", expiresAt: NOW - 2 * HOUR },
    { ...link, userCode: "BBBBBBBB", expiresAt: NOW - 30 * MINUTE },
  ]);
  await recordEvent({ type: "created", actor: "system", at: NOW - 91 * DAY });
  await recordEvent({ type: "queued", actor: "system", at: NOW - 89 * DAY });

  const result = await sweep();
  assert.equal(result.expiredLinks, 1);
  assert.equal(result.prunedEvents, 1);
  const codes = await db.select({ userCode: deviceLinks.userCode }).from(deviceLinks);
  assert.deepEqual(codes.map((row) => row.userCode), ["BBBBBBBB"]);
  const types = await db.select({ type: events.type }).from(events);
  assert.deepEqual(types.map((row) => row.type), ["queued"]);
});

test("ARTIFACT_RETENTION_DAYS of 0 or of no number means the default, for the sweeper and for the dates shown", () => {
  for (const value of ["0", "-3", "soon", ""]) {
    const env = { ARTIFACT_RETENTION_DAYS: value };
    assert.equal(loadRetentionConfig(env).artifactRetentionDays, 7);
    assert.equal(loadCloudConfig(env).artifactRetentionDays, 7);
  }
  assert.equal(loadRetentionConfig({ ARTIFACT_RETENTION_DAYS: "3" }).artifactRetentionDays, 3);
});

test("a result file no row points at is removed after an hour; legacy videos and fresh files are not", async () => {
  const userId = await makeUser("luis");
  const jobId = await makeQueuedJob({ userId });
  const kept = await addArtifact({ jobId, name: "kept", uploadedAt: NOW - MINUTE });
  const folder = dirname(kept);
  const orphan = join(folder, "orphan.bin");
  const arriving = join(folder, "arriving.bin");
  const legacyVideo = join(folder, "legacy.mp4");
  const promoted = join(TEST_DIR, "uploads", "legacy-job", "artifacts", "promoted.bin");
  await mkdir(dirname(promoted), { recursive: true });
  const old = new Date(NOW - 2 * HOUR);
  for (const path of [kept, orphan, legacyVideo, promoted]) {
    await writeFile(path, "video-bytes");
    await utimes(path, old, old);
  }
  await writeFile(arriving, "video-bytes");
  await utimes(arriving, new Date(NOW - MINUTE), new Date(NOW - MINUTE));
  await db.insert(requests).values({ userId, status: "approved", finalVideoPath: promoted });

  const result = await sweep();
  assert.equal(result.orphanArtifacts, 1);
  assert.deepEqual(
    [await exists(orphan), await exists(kept), await exists(arriving), await exists(legacyVideo), await exists(promoted)],
    [false, true, true, true, true],
  );
});

test("the folders of a job whose only file was an orphan go with it", async () => {
  const orphan = join(TEST_DIR, "uploads", "gone-job", "artifacts", "orphan.bin");
  await mkdir(dirname(orphan), { recursive: true });
  await writeFile(orphan, "video-bytes");
  await utimes(orphan, new Date(NOW - 2 * HOUR), new Date(NOW - 2 * HOUR));

  await sweep();
  assert.equal(await exists(join(TEST_DIR, "uploads", "gone-job")), false);
});

test("an old job whose demo is arriving right now is not dropped as abandoned", async () => {
  const userId = await makeUser("luis");
  const [stale] = await db
    .insert(requests)
    .values({ userId, kind: "short", status: "awaiting_demo", createdAt: new Date(NOW - 25 * HOUR) })
    .returning({ id: requests.id });
  if (!stale) throw new Error("job not created");

  const slot = beginDemoUpload({ jobId: stale.id, userId, bytes: 1000 });
  assert.equal(slot.ok, true);
  assert.equal((await sweep()).reapedAbandoned, 0);
  assert.equal((await getJob(stale.id)).status, "awaiting_demo");

  if (slot.ok) slot.release();
  assert.equal((await sweep()).reapedAbandoned, 1);
});
