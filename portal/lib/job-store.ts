import { randomUUID } from "node:crypto";
import { rm } from "node:fs/promises";

import { and, desc, eq, gte, inArray, isNull, ne, or } from "drizzle-orm";

import { db, transact, type Executor } from "../db/client.ts";
import { requests, userCloud, workers } from "../db/schema.ts";
import { isAdminUser } from "./admin.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import {
  adoptDemo,
  beginDemoUpload,
  demoFileSize,
  demoPath,
  demoTempPath,
  demoUploadsInFlight,
  releaseDemo,
  userDemoBytes,
  withDemoLock,
} from "./demo-store.ts";
import { isCsgoDemoHeader, isDemoHeader } from "./demo-validate.ts";
import { recordEvent } from "./events.ts";
import { apiFailure, contentLength, type ApiFailure } from "./http.ts";
import { parseJobSpec } from "./job-spec.ts";
import {
  ACTIVE_STATUSES,
  LEASED_STATUSES,
  isCloudKind,
  isRecord,
  type CloudAccess,
  type CloudKind,
} from "./job-types.ts";
import { calibration, estimateSeconds, usage24h } from "./queue-policy.ts";
import { invalidateQueueSnapshot, type JobRow } from "./queue-store.ts";
import { hasFreeSpace } from "./storage-guard.ts";
import { isSha256Hex } from "./tokens.ts";
import {
  InvalidContentError,
  UploadTooLargeError,
  streamUploadToFile,
  type StreamUploadResult,
} from "./upload-stream.ts";
import {
  checkCreateLimits,
  effectiveLimits,
  loadAccessDefault,
  loadUserLimits,
  resolveAccess,
  type UserLimits,
} from "./user-limits.ts";
import type { UserContext } from "./user-context.ts";
import { parseWorkerHealth } from "./worker-protocol.ts";

const DAY_MS = 24 * 60 * 60 * 1000;
const LIST_WINDOW_MS = 14 * DAY_MS;
const LIST_LIMIT = 100;
const MAX_TITLE_CHARS = 120;
const MAX_FILE_NAME_CHARS = 200;

export interface UserCloudState {
  access: CloudAccess;
  limits: UserLimits;
}

// The user's cloud access and effective limits: env defaults plus their user_cloud row.
export async function userCloudState(userId: string): Promise<UserCloudState> {
  const [row] = await db.select().from(userCloud).where(eq(userCloud.userId, userId));
  return {
    access: resolveAccess({
      stored: row?.access ?? null,
      isAdmin: await isAdminUser(userId),
      accessDefault: loadAccessDefault(),
    }),
    limits: effectiveLimits(loadUserLimits(), row ?? null),
  };
}

export interface UserUsage {
  active: number;
  activeFullDemo: number;
  secondsLast24h: number;
  // Estimates of the jobs still waiting (queued and awaiting_demo).
  secondsCommitted: number;
}

export async function userUsage(
  executor: Executor,
  options: { userId: string; now: number },
): Promise<UserUsage> {
  const { userId, now } = options;
  const rows = await executor
    .select({
      userId: requests.userId,
      kind: requests.kind,
      status: requests.status,
      finishedAt: requests.finishedAt,
      machineSeconds: requests.machineSeconds,
      estimatedSeconds: requests.estimatedSeconds,
      failureCode: requests.failureCode,
      canceledBy: requests.canceledBy,
    })
    .from(requests)
    .where(
      and(
        eq(requests.userId, userId),
        ne(requests.kind, "manual"),
        or(
          inArray(requests.status, [...ACTIVE_STATUSES]),
          gte(requests.finishedAt, now - DAY_MS),
        ),
      ),
    );
  const active = rows.filter((row) => ACTIVE_STATUSES.some((status) => status === row.status));
  const waiting = active.filter((row) => !LEASED_STATUSES.some((status) => status === row.status));
  return {
    active: active.length,
    activeFullDemo: active.filter((row) => row.kind === "full_demo").length,
    secondsLast24h: usage24h(rows, now),
    secondsCommitted: waiting.reduce((sum, row) => sum + (row.estimatedSeconds ?? 0), 0),
  };
}

async function calibrationFor(kind: CloudKind): Promise<number> {
  const rows = await db
    .select({
      machineSeconds: requests.machineSeconds,
      estCaptureSeconds: requests.estCaptureSeconds,
    })
    .from(requests)
    .where(and(eq(requests.kind, kind), eq(requests.status, "done")))
    .orderBy(desc(requests.finishedAt))
    .limit(30);
  return calibration(
    rows.map((row) => ({
      machineSeconds: row.machineSeconds,
      uncalibratedSeconds: estimateSeconds({
        kind,
        captureSeconds: row.estCaptureSeconds ?? 0,
        calibration: 1,
      }),
    })),
  );
}

export type CreateJobResult =
  | {
      ok: true;
      id: string;
      status: "queued" | "awaiting_demo";
      demoUpload: "present" | "required";
    }
  | ApiFailure;

export interface CreateJobInput {
  user: UserContext;
  body: unknown;
  now: number;
}

function invalidSpec(error: string): ApiFailure {
  return { ...apiFailure(422, "invalid_spec"), error };
}

function storedHealth(raw: string | null): unknown {
  try {
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

// The kill plan schema of the worker seen last, or "" when no worker has reported one.
async function workerPlanSchema(): Promise<string> {
  const [worker] = await db
    .select({ health: workers.health })
    .from(workers)
    .where(isNull(workers.revokedAt))
    .orderBy(desc(workers.lastSeenAt))
    .limit(1);
  return parseWorkerHealth(storedHealth(worker?.health ?? null)).planSchema;
}

// True when the Studio that built the plan cuts segments differently from the worker's.
async function planSchemaMismatch(client: Record<string, unknown> | undefined): Promise<boolean> {
  const planSchema = client?.planSchema;
  if (typeof planSchema !== "string" || planSchema === "") return false;
  const expected = await workerPlanSchema();
  return expected !== "" && expected !== planSchema;
}

// Creates a cloud job. Every limit is checked here, before a byte of demo is uploaded.
export async function createJob(input: CreateJobInput): Promise<CreateJobResult> {
  const { user, body, now } = input;
  const config = loadCloudConfig();
  if (!isRecord(body)) return apiFailure(400, "invalid_request");

  const { kind, demo } = body;
  if (!isCloudKind(kind)) return invalidSpec("kind must be short or full_demo");
  if (!config.kinds.includes(kind)) return apiFailure(503, "kind_unavailable");

  const state = await userCloudState(user.userId);
  if (state.access !== "allowed") return apiFailure(403, `cloud_access_${state.access}`);

  if (!isRecord(demo) || !isSha256Hex(demo.sha256)) {
    return invalidSpec("demo.sha256 must be 64 lowercase hex characters");
  }
  const { sha256, sizeBytes } = demo;
  if (typeof sizeBytes !== "number" || !Number.isSafeInteger(sizeBytes) || sizeBytes < 1) {
    return invalidSpec("demo.sizeBytes must be a positive integer");
  }
  if (sizeBytes > config.maxDemoBytes) return apiFailure(413, "demo_too_large");

  const parsed = parseJobSpec(body.spec, kind);
  if (!parsed.ok) return invalidSpec(parsed.error);
  if (await planSchemaMismatch(parsed.spec.client)) {
    return apiFailure(409, "studio_version_mismatch");
  }

  const estimatedSeconds = estimateSeconds({
    kind,
    captureSeconds: parsed.captureSeconds,
    calibration: await calibrationFor(kind),
  });
  if (!(await hasFreeSpace())) return apiFailure(503, "cloud_storage_full");

  const title = typeof body.title === "string" ? body.title.slice(0, MAX_TITLE_CHARS) : null;
  const fileName =
    typeof demo.fileName === "string" && demo.fileName.length > 0
      ? demo.fileName.slice(0, MAX_FILE_NAME_CHARS)
      : "demo.dem";
  const storedPath = demoPath(user.userId, sha256);

  const result = await withDemoLock(async (): Promise<CreateJobResult> => {
    const storedSize = await demoFileSize(storedPath);
    const present = storedSize !== null;

    return transact(async (tx): Promise<CreateJobResult> => {
      const usage = await userUsage(tx, { userId: user.userId, now });
      const violation = checkCreateLimits({
        limits: state.limits,
        kind,
        active: usage.active,
        activeFullDemo: usage.activeFullDemo,
        usedSeconds: usage.secondsLast24h,
        committedSeconds: usage.secondsCommitted,
        estimatedSeconds,
      });
      if (violation) return apiFailure(429, violation);
      const demoBytes = await userDemoBytes(tx, {
        userId: user.userId,
        incoming: { sha256, sizeBytes: storedSize ?? sizeBytes },
      });
      if (demoBytes > config.maxDemoBytesPerUser) return apiFailure(429, "limit_storage");

      const waiting = await tx
        .select({ id: requests.id })
        .from(requests)
        .where(
          and(inArray(requests.status, ["queued", "awaiting_demo"]), ne(requests.kind, "manual")),
        );
      if (waiting.length >= config.maxQueuedJobs) return apiFailure(503, "cloud_queue_full");

      const status = present ? "queued" : "awaiting_demo";
      const [created] = await tx
        .insert(requests)
        .values({
          userId: user.userId,
          deviceId: user.deviceId,
          status,
          kind,
          title,
          spec: JSON.stringify(parsed.spec),
          targetSteamId: parsed.spec.targetSteamId,
          demoSha256: sha256,
          demoSizeBytes: storedSize ?? sizeBytes,
          demoOriginalName: fileName,
          demoPath: present ? storedPath : null,
          estCaptureSeconds: Math.round(parsed.captureSeconds),
          estimatedSeconds,
          enqueuedAt: present ? now : null,
          createdAt: new Date(now),
          updatedAt: new Date(now),
        })
        .returning({ id: requests.id });
      if (!created) return apiFailure(500, "internal");

      const event = {
        actor: `user:${user.userId}`,
        at: now,
        requestId: created.id,
        subjectUserId: user.userId,
      };
      await recordEvent({ ...event, type: "created", detail: `${kind}, ${estimatedSeconds} s` }, tx);
      if (present) await recordEvent({ ...event, type: "queued", detail: "demo already stored" }, tx);
      return {
        ok: true,
        id: created.id,
        status,
        demoUpload: present ? "present" : "required",
      };
    });
  });
  if (result.ok) invalidateQueueSnapshot();
  return result;
}

export interface AttachDemoInput {
  user: UserContext;
  jobId: string;
  request: Request;
}

// Receives the demo of an awaiting_demo job and puts the job in the queue.
export async function attachDemo(
  input: AttachDemoInput,
): Promise<{ ok: true } | ApiFailure> {
  const { user, jobId, request } = input;
  const config = loadCloudConfig();
  const [job] = await db
    .select()
    .from(requests)
    .where(and(eq(requests.id, jobId), eq(requests.userId, user.userId)));
  if (!job || !isCloudKind(job.kind) || !job.demoSha256) return apiFailure(404, "not_found");
  if (job.status !== "awaiting_demo") return apiFailure(409, "invalid_state");

  const declared = contentLength(request);
  if (declared === null || !request.body) return apiFailure(411, "length_required");
  if (declared > config.maxDemoBytes) return apiFailure(413, "demo_too_large");

  const slot = beginDemoUpload({ jobId: job.id, userId: user.userId, bytes: declared });
  if (!slot.ok) {
    return apiFailure(slot.code === "upload_in_progress" ? 409 : 429, slot.code);
  }
  try {
    return await receiveDemo({ user, job, body: request.body, declared });
  } finally {
    slot.release();
  }
}

interface ReceiveDemoInput {
  user: UserContext;
  job: JobRow;
  body: ReadableStream<Uint8Array>;
  // The Content-Length, already reserved by the caller's upload slot.
  declared: number;
}

async function receiveDemo(input: ReceiveDemoInput): Promise<{ ok: true } | ApiFailure> {
  const { user, job, declared } = input;
  const config = loadCloudConfig();
  if (!job.demoSha256) return apiFailure(404, "not_found");

  const demoBytes = await userDemoBytes(db, {
    userId: user.userId,
    incoming: { sha256: job.demoSha256, sizeBytes: declared },
  });
  if (demoBytes > config.maxDemoBytesPerUser) return apiFailure(429, "limit_storage");
  // Every upload in flight, this one included, will take its declared length from the volume.
  const reservedBytes = demoUploadsInFlight().reduce((sum, upload) => sum + upload.bytes, 0);
  if (!(await hasFreeSpace({ reservedBytes }))) return apiFailure(503, "cloud_storage_full");

  const tempPath = demoTempPath(user.userId, randomUUID());
  let upload: StreamUploadResult;
  try {
    upload = await streamUploadToFile({
      body: input.body,
      destPath: tempPath,
      // Never more than was declared, because that is what was reserved.
      maxBytes: declared,
      validateHeader: isDemoHeader,
    });
  } catch (err) {
    if (err instanceof UploadTooLargeError) return apiFailure(413, "demo_too_large");
    if (err instanceof InvalidContentError) {
      return apiFailure(415, isCsgoDemoHeader(err.header) ? "csgo_demo" : "not_a_demo");
    }
    throw err;
  }
  if (upload.sha256 !== job.demoSha256) {
    await rm(tempPath, { force: true });
    return apiFailure(422, "sha256_mismatch");
  }

  // The upload can take minutes; the job enters the queue when it ends, not when it began.
  const now = Date.now();
  const finalPath = demoPath(user.userId, job.demoSha256);
  const queued = await withDemoLock(async () => {
    await adoptDemo(tempPath, finalPath);
    return transact(async (tx) => {
      // The status guard keeps a job cancelled during the upload out of the queue.
      const updated = await tx
        .update(requests)
        .set({
          status: "queued",
          demoPath: finalPath,
          demoSizeBytes: upload.bytesWritten,
          enqueuedAt: now,
          updatedAt: new Date(now),
        })
        .where(and(eq(requests.id, job.id), eq(requests.status, "awaiting_demo")))
        .returning({ id: requests.id });
      if (updated.length === 0) return false;
      const event = {
        actor: `user:${user.userId}`,
        at: now,
        requestId: job.id,
        subjectUserId: user.userId,
      };
      await recordEvent({ ...event, type: "demo_uploaded", detail: `${upload.bytesWritten} bytes` }, tx);
      await recordEvent({ ...event, type: "queued" }, tx);
      return true;
    });
  });
  if (!queued) {
    // Cancelled while it uploaded: the file was stored for nobody, so it does not stay.
    await releaseDemo(finalPath, now);
    return apiFailure(409, "invalid_state");
  }
  invalidateQueueSnapshot();
  return { ok: true };
}

// The user's own cloud job, or null. Another user's id looks exactly like a missing one.
export async function findUserJob(userId: string, jobId: string): Promise<JobRow | null> {
  const [job] = await db
    .select()
    .from(requests)
    .where(
      and(eq(requests.id, jobId), eq(requests.userId, userId), ne(requests.kind, "manual")),
    );
  return job ?? null;
}

// Every active job plus the ones touched in the last 14 days, newest first.
export async function listUserJobs(userId: string, now: number): Promise<JobRow[]> {
  return db
    .select()
    .from(requests)
    .where(
      and(
        eq(requests.userId, userId),
        ne(requests.kind, "manual"),
        or(
          inArray(requests.status, [...ACTIVE_STATUSES]),
          gte(requests.updatedAt, new Date(now - LIST_WINDOW_MS)),
        ),
      ),
    )
    .orderBy(desc(requests.createdAt))
    .limit(LIST_LIMIT);
}
