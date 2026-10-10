import { rm } from "node:fs/promises";

import { and, eq, gte, inArray, isNull, ne, or, sql } from "drizzle-orm";

import { db, transact, type Executor, type Tx } from "../db/client.ts";
import { requestArtifacts, requests, userCloud, users, workers } from "../db/schema.ts";
import { adminUserIds } from "./admin.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import { demoFileSize, releaseDemo, withDemoLock } from "./demo-store.ts";
import { recordEvent } from "./events.ts";
import { classOf, decideFailure } from "./failure-policy.ts";
import { readStoredSpec } from "./job-spec.ts";
import {
  LEASED_STATUSES,
  isCloudKind,
  isLegacyStatus,
  type CancelActor,
  type CloudKind,
  type FailureCode,
  type QueueState,
} from "./job-types.ts";
import {
  isWorkerOnline,
  maxRuntimeSeconds,
  pickNext,
  queueStateOf,
  selectCandidates,
  simulateQueue,
  usageByUser,
  type QueueJob,
  type QueuePlacement,
  type RunningJob,
} from "./queue-policy.ts";
import { hasFreeSpace } from "./storage-guard.ts";
import type { FailBody, HeartbeatBody, PhaseBody } from "./worker-protocol.ts";

export type JobRow = typeof requests.$inferSelect;
export type WorkerRow = typeof workers.$inferSelect;

const DAY_MS = 24 * 60 * 60 * 1000;
// A worker that restarts resumes its uploads, so an uploading job gets this long.
const UPLOAD_GRACE_MS = 30 * 60 * 1000;
// A worker that keeps heartbeating can still be stuck; past these the portal ends the attempt.
const RUNTIME_MARGIN_MS = 300 * 1000;
const UPLOAD_LIMIT_MS = 6.5 * 60 * 60 * 1000;
const SNAPSHOT_TTL_MS = 5000;
const LEASE_SWEEP_INTERVAL_MS = 30 * 1000;
const WORKER_LOST_MESSAGE =
  "La máquina de la nube dejó de responder mientras procesaba tu vídeo.";
// The worker words a machine fault for a job that returns to the queue; a failed one needs this.
const MACHINE_GAVE_UP_MESSAGE =
  "La nube intentó grabar este vídeo varias veces y no lo consiguió. Puede deberse a esta demo.";
// What /me simulates as "a new typical job" per kind.
const TYPICAL_SECONDS: Record<CloudKind, number> = { short: 480, full_demo: 3600 };

async function removeFiles(paths: string[]): Promise<void> {
  for (const path of paths) await rm(path, { force: true });
}

function toQueueJob(row: JobRow): QueueJob | null {
  if (!isCloudKind(row.kind) || row.enqueuedAt === null || row.estimatedSeconds === null) {
    return null;
  }
  return {
    id: row.id,
    userId: row.userId,
    kind: row.kind,
    enqueuedAt: row.enqueuedAt,
    estimatedSeconds: row.estimatedSeconds,
    priorityBoost: row.priorityBoost,
  };
}

interface QueueInputs {
  queued: QueueJob[];
  running: RunningJob[];
  usage: Map<string, number>;
  blockedUserIds: Set<string>;
}

async function loadQueueInputs(
  executor: Executor,
  options: { now: number; admins: ReadonlySet<string> },
): Promise<QueueInputs> {
  const { now, admins } = options;
  const queuedRows = await executor.select().from(requests).where(eq(requests.status, "queued"));
  const usageRows = await executor
    .select({
      userId: requests.userId,
      status: requests.status,
      finishedAt: requests.finishedAt,
      machineSeconds: requests.machineSeconds,
      estimatedSeconds: requests.estimatedSeconds,
      failureCode: requests.failureCode,
      canceledBy: requests.canceledBy,
      claimedAt: requests.claimedAt,
    })
    .from(requests)
    .where(
      or(
        inArray(requests.status, [...LEASED_STATUSES]),
        gte(requests.finishedAt, now - DAY_MS),
      ),
    );
  const blockedRows = await executor
    .select({ userId: userCloud.userId })
    .from(userCloud)
    .where(eq(userCloud.access, "blocked"));

  const queued: QueueJob[] = [];
  for (const row of queuedRows) {
    const job = toQueueJob(row);
    if (job) queued.push(job);
  }
  const running: RunningJob[] = [];
  for (const row of usageRows) {
    if (row.status !== "running") continue;
    running.push({
      estimatedSeconds: row.estimatedSeconds ?? 0,
      claimedAt: row.claimedAt?.getTime() ?? now,
    });
  }
  return {
    queued,
    running,
    usage: usageByUser(usageRows, now),
    blockedUserIds: new Set(
      blockedRows.map((row) => row.userId).filter((userId) => !admins.has(userId)),
    ),
  };
}

export interface QueueSnapshot {
  state: QueueState;
  // Every queued job, in the order the scheduler will take them.
  placements: Map<string, QueuePlacement>;
  queuedCount: number;
  eligible: QueueJob[];
  running: RunningJob[];
  usage: Map<string, number>;
}

declare global {
  // On globalThis so every route bundle reads, and invalidates, the same cache.
  var cliphubQueueSnapshot: { snapshot: QueueSnapshot; expiresAt: number } | undefined;
}

export function invalidateQueueSnapshot(): void {
  globalThis.cliphubQueueSnapshot = undefined;
}

// Positions and start estimates for the whole queue, cached for 5 seconds.
export async function queueSnapshot(now: number): Promise<QueueSnapshot> {
  const cached = globalThis.cliphubQueueSnapshot;
  if (cached && cached.expiresAt > now) return cached.snapshot;

  const { kinds } = loadCloudConfig();
  const admins = await adminUserIds();
  const inputs = await loadQueueInputs(db, { now, admins });
  const workerRows = await db.select().from(workers);
  const state = queueStateOf(workerRows, now);

  const isEligible = (job: QueueJob) =>
    kinds.includes(job.kind) && !inputs.blockedUserIds.has(job.userId);
  const eligible = inputs.queued.filter(isEligible);
  const placements = simulateQueue({
    queued: eligible,
    running: inputs.running,
    usageByUser: inputs.usage,
    now,
    queueState: state,
  });
  // Jobs the scheduler will not take (blocked user, kind switched off) keep a place at the end.
  for (const job of inputs.queued) {
    if (placements.has(job.id)) continue;
    placements.set(job.id, {
      position: placements.size + 1,
      estimatedStartAt: null,
      estimatedDoneAt: null,
    });
  }

  const snapshot: QueueSnapshot = {
    state,
    placements,
    queuedCount: inputs.queued.length,
    eligible,
    running: inputs.running,
    usage: inputs.usage,
  };
  globalThis.cliphubQueueSnapshot = { snapshot, expiresAt: now + SNAPSHOT_TTL_MS };
  return snapshot;
}

export interface WaitInput {
  snapshot: QueueSnapshot;
  userId: string;
  kind: CloudKind;
  now: number;
}

// How long a new typical job of this user would wait to start, or null when the queue is not online.
export function estimateWaitSeconds(input: WaitInput): number | null {
  const { snapshot, userId, kind, now } = input;
  const probe: QueueJob = {
    id: "new-job-probe",
    userId,
    kind,
    enqueuedAt: now,
    estimatedSeconds: TYPICAL_SECONDS[kind],
    priorityBoost: 0,
  };
  const placement = simulateQueue({
    queued: [...snapshot.eligible, probe],
    running: snapshot.running,
    usageByUser: snapshot.usage,
    now,
    queueState: snapshot.state,
  }).get(probe.id);
  if (!placement || placement.estimatedStartAt === null) return null;
  return Math.max(0, Math.round((placement.estimatedStartAt - now) / 1000));
}

interface FailureApplication {
  job: JobRow;
  // The worker reporting the failure; null when the portal noticed an expired lease.
  worker: WorkerRow | null;
  code: FailureCode;
  message: string;
  detail: string;
  machineSeconds: number;
  now: number;
}

interface FailureOutcome {
  outcome: "requeued" | "failed" | "canceled";
  workerPaused: boolean;
  removedPaths: string[];
}

// Applies the failure policy to a leased job. "invalid" is a cancel nobody asked for.
async function applyFailure(
  tx: Tx,
  input: FailureApplication,
): Promise<FailureOutcome | "invalid" | "lease_lost"> {
  const { job, worker, code, now } = input;
  const decision = decideFailure({
    job,
    code,
    source: worker ? "worker" : "lease",
    detail: input.detail,
    consecutiveFailures: worker?.consecutiveFailures ?? 0,
  });
  if (!decision.ok) return "invalid";

  const machineFault = classOf(code) === "machine";
  // The machine's own faults are not charged to the user.
  const charged = machineFault ? 0 : input.machineSeconds;
  const userMessage = machineFault ? MACHINE_GAVE_UP_MESSAGE : input.message;
  const shared = {
    attempt: decision.attempt,
    machineRequeues: decision.machineRequeues,
    leaseExpiresAt: null,
    stage: null,
    machineSeconds: job.machineSeconds + charged,
    updatedAt: new Date(now),
  };
  const next =
    decision.status === "queued"
      ? {
          ...shared,
          status: "queued",
          workerId: null,
          claimedAt: null,
          uploadingAt: null,
          localJobId: null,
          progressPercent: null,
          progressDetail: null,
        }
      : decision.status === "failed"
        ? {
            ...shared,
            status: "failed",
            finishedAt: now,
            priorityBoost: 0,
            failureCode: code,
            failureReason: userMessage.slice(0, 500),
            failureDetail: input.detail.slice(0, 4000),
          }
        : { ...shared, status: "canceled", finishedAt: now, priorityBoost: 0 };

  const updated = await tx
    .update(requests)
    .set(next)
    .where(and(eq(requests.id, job.id), eq(requests.status, job.status)))
    .returning({ id: requests.id });
  if (updated.length === 0) return "lease_lost";

  // Whatever was uploaded belongs to the attempt that just ended.
  const removed = await tx
    .delete(requestArtifacts)
    .where(eq(requestArtifacts.requestId, job.id))
    .returning({ path: requestArtifacts.path });

  const actor = worker ? `worker:${worker.id}` : "system";
  const base = { at: now, actor, requestId: job.id, workerId: job.workerId, subjectUserId: job.userId };
  if (!worker) await recordEvent({ ...base, type: "lease_expired" }, tx);
  await recordEvent({ ...base, type: decision.outcome, detail: `${code}: ${input.detail}` }, tx);

  if (worker) {
    const paused = decision.pauseReason !== null;
    await tx
      .update(workers)
      .set({
        consecutiveFailures: decision.consecutiveFailures,
        ...(paused
          ? { paused: true, pausedBy: "auto", pauseReason: decision.pauseReason }
          : {}),
      })
      .where(eq(workers.id, worker.id));
    if (paused) {
      await recordEvent(
        { ...base, type: "worker_auto_paused", actor: "system", detail: decision.pauseReason },
        tx,
      );
    }
  }

  return {
    outcome: decision.outcome,
    workerPaused: decision.pauseReason !== null,
    removedPaths: removed.map((row) => row.path),
  };
}

function leaseExpired(job: JobRow, now: number): boolean {
  if (job.leaseExpiresAt === null) return false;
  if (job.status === "running") return job.leaseExpiresAt < now;
  return job.status === "uploading" && job.leaseExpiresAt + UPLOAD_GRACE_MS < now;
}

// Why the portal gives up on a leased job, or null while its worker may keep it.
function lostCause(job: JobRow, now: number): string | null {
  if (leaseExpired(job, now)) {
    return `lease expired at ${new Date(job.leaseExpiresAt ?? now).toISOString()}`;
  }
  if (job.status === "running" && job.claimedAt !== null) {
    const limitSeconds = maxRuntimeSeconds(job.estimatedSeconds ?? 0);
    const overdue = now - job.claimedAt.getTime() > limitSeconds * 1000 + RUNTIME_MARGIN_MS;
    return overdue ? `running past its limit of ${limitSeconds} s` : null;
  }
  if (job.status === "uploading") {
    const since = job.uploadingAt ?? job.updatedAt.getTime();
    return now - since > UPLOAD_LIMIT_MS ? "uploading past its limit of 23400 s" : null;
  }
  return null;
}

export interface SweepOptions {
  // Jobs this worker holds are left alone: its claim proves it is alive.
  exceptWorkerId?: string;
}

// Requeues or fails jobs whose worker went silent or stopped making progress. Returns how many.
export async function sweepLeases(now: number, options: SweepOptions = {}): Promise<number> {
  const { exceptWorkerId } = options;
  const leased = await db
    .select()
    .from(requests)
    .where(
      and(
        inArray(requests.status, [...LEASED_STATUSES]),
        exceptWorkerId === undefined
          ? undefined
          : or(isNull(requests.workerId), ne(requests.workerId, exceptWorkerId)),
      ),
    );
  // Only a job that looks lost is worth a write transaction; it is read again inside it.
  const candidates = leased.filter((job) => lostCause(job, now) !== null);

  let swept = 0;
  for (const candidate of candidates) {
    const removedPaths = await transact(async (tx) => {
      const [job] = await tx.select().from(requests).where(eq(requests.id, candidate.id));
      if (!job || job.workerId === exceptWorkerId) return null;
      const detail = lostCause(job, now);
      if (detail === null) return null;
      const result = await applyFailure(tx, {
        job,
        worker: null,
        code: "worker_lost",
        message: WORKER_LOST_MESSAGE,
        detail,
        machineSeconds: 0,
        now,
      });
      return typeof result === "string" ? null : result.removedPaths;
    });
    if (removedPaths) {
      swept += 1;
      await removeFiles(removedPaths);
    }
  }
  if (swept > 0) invalidateQueueSnapshot();
  return swept;
}

export interface WorkerJob {
  id: string;
  kind: string;
  attempt: number;
  maxRuntimeSeconds: number;
  leaseExpiresAt: number;
  demo: { sha256: string; sizeBytes: number; fileName: string };
  submitterLabel: string;
  title: string;
  spec: Record<string, unknown>;
}

export type ClaimRefusal = "empty" | "paused" | "busy" | "upload_backlog" | "portal_storage_full";

export type ClaimResult =
  | { claimed: false; reason: ClaimRefusal }
  | { claimed: true; job: WorkerJob };

export interface ClaimInput {
  worker: WorkerRow;
  kinds: CloudKind[];
  now: number;
}

// Hands the worker the next job. One capture at a time per worker.
export async function claimNext(input: ClaimInput): Promise<ClaimResult> {
  const { worker, now } = input;
  // Never this worker's own jobs: taking one away and handing it back in one call loses its upload.
  await sweepLeases(now, { exceptWorkerId: worker.id });

  const config = loadCloudConfig();
  const kinds = input.kinds.filter((kind) => config.kinds.includes(kind));
  const admins = await adminUserIds();
  const storageOk = await hasFreeSpace({ floorBytes: config.minFreeBytesClaim });

  const result = await transact(async (tx): Promise<ClaimResult> => {
    const [current] = await tx.select().from(workers).where(eq(workers.id, worker.id));
    if (!current || current.revokedAt !== null || current.paused) {
      return { claimed: false, reason: "paused" };
    }
    const held = await tx
      .select({ status: requests.status })
      .from(requests)
      .where(
        and(eq(requests.workerId, worker.id), inArray(requests.status, [...LEASED_STATUSES])),
      );
    if (held.some((job) => job.status === "running")) return { claimed: false, reason: "busy" };
    // Captures that pile up behind a stalled upload would all be lost together.
    if (held.length >= config.maxUploadBacklog) return { claimed: false, reason: "upload_backlog" };
    if (!storageOk) return { claimed: false, reason: "portal_storage_full" };

    const inputs = await loadQueueInputs(tx, { now, admins });
    let candidates = selectCandidates({
      queued: inputs.queued,
      kinds,
      blockedUserIds: inputs.blockedUserIds,
    });
    const leaseExpiresAt = now + config.leaseSeconds * 1000;
    for (;;) {
      const next = pickNext({ candidates, usageByUser: inputs.usage, now });
      if (!next) return { claimed: false, reason: "empty" };
      // The status guard is what makes the claim atomic: only one caller can win this row.
      const [claimed] = await tx
        .update(requests)
        .set({
          status: "running",
          stage: "downloading",
          workerId: worker.id,
          attempt: sql`${requests.attempt} + 1`,
          claimedAt: new Date(now),
          leaseExpiresAt,
          progressPercent: null,
          progressDetail: null,
          updatedAt: new Date(now),
        })
        .where(and(eq(requests.id, next.id), eq(requests.status, "queued")))
        .returning();
      if (!claimed) {
        candidates = candidates.filter((job) => job.id !== next.id);
        continue;
      }
      const [submitter] = await tx
        .select({ name: users.name, email: users.email })
        .from(users)
        .where(eq(users.id, claimed.userId));
      await recordEvent(
        {
          type: "claimed",
          actor: `worker:${worker.id}`,
          at: now,
          requestId: claimed.id,
          workerId: worker.id,
          subjectUserId: claimed.userId,
          detail: `attempt ${claimed.attempt}`,
        },
        tx,
      );
      return {
        claimed: true,
        job: {
          id: claimed.id,
          kind: claimed.kind,
          attempt: claimed.attempt,
          maxRuntimeSeconds: maxRuntimeSeconds(claimed.estimatedSeconds ?? 0),
          leaseExpiresAt,
          demo: {
            sha256: claimed.demoSha256 ?? "",
            sizeBytes: claimed.demoSizeBytes ?? 0,
            fileName: claimed.demoOriginalName ?? "demo.dem",
          },
          submitterLabel: submitter?.name ?? submitter?.email ?? "unknown",
          title: claimed.title ?? "",
          spec: readStoredSpec(claimed.spec) ?? {},
        },
      };
    }
  });
  if (result.claimed) invalidateQueueSnapshot();
  return result;
}

export interface HeartbeatJobResult {
  id: string;
  lease: "ok" | "lost";
  leaseExpiresAt: number;
  cancelRequested: boolean;
}

export interface HeartbeatResult {
  now: number;
  paused: boolean;
  pauseReason: string | null;
  jobs: HeartbeatJobResult[];
}

export interface HeartbeatInput {
  worker: WorkerRow;
  body: HeartbeatBody;
  now: number;
}

// Records the worker's health and extends the lease of every job it still holds.
export async function recordHeartbeat(input: HeartbeatInput): Promise<HeartbeatResult> {
  const { worker, body, now } = input;
  const leaseExpiresAt = now + loadCloudConfig().leaseSeconds * 1000;
  // A worker coming back, or changing state, changes what the queue can promise.
  if (!isWorkerOnline(worker, now) || worker.state !== body.state) invalidateQueueSnapshot();

  return transact(async (tx) => {
    const [current] = await tx
      .update(workers)
      .set({
        lastSeenAt: now,
        state: body.state,
        blockedCode: body.blocked?.code ?? null,
        blockedDetail: body.blocked?.detail ?? null,
        health: JSON.stringify(body.health),
      })
      .where(eq(workers.id, worker.id))
      .returning({ paused: workers.paused, pauseReason: workers.pauseReason });

    const jobs: HeartbeatJobResult[] = [];
    for (const report of body.jobs) {
      const [job] = await tx
        .select()
        .from(requests)
        .where(
          and(
            eq(requests.id, report.id),
            eq(requests.workerId, worker.id),
            inArray(requests.status, [...LEASED_STATUSES]),
          ),
        );
      // An entry from another attempt of the same job renews nothing.
      if (!job || report.attempt !== job.attempt) {
        jobs.push({ id: report.id, lease: "lost", leaseExpiresAt: 0, cancelRequested: false });
        continue;
      }
      // A report for the phase the job already left carries stale progress.
      const samePhase = job.status === report.phase;
      const stageChanged = samePhase && report.stage !== null && report.stage !== job.stage;
      await tx
        .update(requests)
        .set({
          leaseExpiresAt,
          localJobId: report.localJobId ?? job.localJobId,
          ...(samePhase
            ? { progressPercent: report.percent, progressDetail: report.detail }
            : {}),
          ...(stageChanged ? { stage: report.stage, updatedAt: new Date(now) } : {}),
        })
        .where(eq(requests.id, job.id));
      if (stageChanged) {
        await recordEvent(
          {
            type: "stage",
            actor: `worker:${worker.id}`,
            at: now,
            requestId: job.id,
            workerId: worker.id,
            subjectUserId: job.userId,
            detail: report.stage,
          },
          tx,
        );
      }
      jobs.push({
        id: job.id,
        lease: "ok",
        leaseExpiresAt,
        cancelRequested: job.cancelRequestedAt !== null,
      });
    }
    return {
      now,
      paused: current?.paused ?? false,
      pauseReason: current?.pauseReason ?? null,
      jobs,
    };
  });
}

export interface LeaseQuery {
  jobId: string;
  // The attempt the caller is working on: the fencing token of the claim.
  attempt: number;
  statuses: readonly string[];
}

// The job, only when this attempt of it is leased to this worker in one of the given statuses.
export async function leasedJob(worker: WorkerRow, query: LeaseQuery): Promise<JobRow | null> {
  const [job] = await db
    .select()
    .from(requests)
    .where(
      and(
        eq(requests.id, query.jobId),
        eq(requests.workerId, worker.id),
        eq(requests.attempt, query.attempt),
        inArray(requests.status, [...query.statuses]),
      ),
    );
  return job ?? null;
}

export interface PhaseInput {
  worker: WorkerRow;
  jobId: string;
  attempt: number;
  body: PhaseBody;
  now: number;
}

// running to uploading: frees the capture slot. False means the lease is gone.
export async function enterUploading(input: PhaseInput): Promise<boolean> {
  const { worker, jobId, attempt, body, now } = input;
  const leaseExpiresAt = now + loadCloudConfig().leaseSeconds * 1000;

  const entered = await transact(async (tx) => {
    const [job] = await tx
      .update(requests)
      .set({
        status: "uploading",
        uploadingAt: now,
        stage: null,
        progressPercent: 0,
        progressDetail: null,
        machineSeconds: sql`${requests.machineSeconds} + ${body.machineSeconds}`,
        localJobId: body.localJobId ?? sql`${requests.localJobId}`,
        leaseExpiresAt,
        updatedAt: new Date(now),
      })
      .where(
        and(
          eq(requests.id, jobId),
          eq(requests.workerId, worker.id),
          eq(requests.attempt, attempt),
          eq(requests.status, "running"),
        ),
      )
      .returning({ userId: requests.userId });
    if (!job) return false;
    await recordEvent(
      {
        type: "phase_uploading",
        actor: `worker:${worker.id}`,
        at: now,
        requestId: jobId,
        workerId: worker.id,
        subjectUserId: job.userId,
        detail: `${body.machineSeconds} s`,
      },
      tx,
    );
    return true;
  });
  if (entered) {
    invalidateQueueSnapshot();
    return true;
  }
  // A repeated call after a lost response must not look like a lost lease.
  return (await leasedJob(worker, { jobId, attempt, statuses: ["uploading"] })) !== null;
}

export interface WorkerJobInput {
  worker: WorkerRow;
  jobId: string;
  attempt: number;
  now: number;
}

// uploading to done. Needs at least one complete video.
export async function completeJob(
  input: WorkerJobInput,
): Promise<"ok" | "lease_lost" | "no_artifacts"> {
  const { worker, jobId, attempt, now } = input;
  const result = await transact(async (tx) => {
    const [job] = await tx
      .select()
      .from(requests)
      .where(
        and(
          eq(requests.id, jobId),
          eq(requests.workerId, worker.id),
          eq(requests.attempt, attempt),
        ),
      );
    if (job?.status === "done") return { status: "ok" as const, removed: [] };
    if (!job || job.status !== "uploading") return { status: "lease_lost" as const, removed: [] };

    const [video] = await tx
      .select({ id: requestArtifacts.id })
      .from(requestArtifacts)
      .where(
        and(
          eq(requestArtifacts.requestId, jobId),
          eq(requestArtifacts.status, "ready"),
          eq(requestArtifacts.kind, "video"),
        ),
      )
      .limit(1);
    if (!video) return { status: "no_artifacts" as const, removed: [] };

    const unfinished = await tx
      .delete(requestArtifacts)
      .where(and(eq(requestArtifacts.requestId, jobId), ne(requestArtifacts.status, "ready")))
      .returning({ path: requestArtifacts.path });
    await tx
      .update(requests)
      .set({
        status: "done",
        finishedAt: now,
        priorityBoost: 0,
        leaseExpiresAt: null,
        progressPercent: 100,
        progressDetail: null,
        cancelRequestedAt: null,
        canceledBy: null,
        updatedAt: new Date(now),
      })
      .where(eq(requests.id, jobId));
    await tx.update(workers).set({ consecutiveFailures: 0 }).where(eq(workers.id, worker.id));
    await recordEvent(
      {
        type: "done",
        actor: `worker:${worker.id}`,
        at: now,
        requestId: jobId,
        workerId: worker.id,
        subjectUserId: job.userId,
      },
      tx,
    );
    return { status: "ok" as const, removed: unfinished.map((row) => row.path) };
  });
  await removeFiles(result.removed);
  invalidateQueueSnapshot();
  return result.status;
}

export interface FailInput extends WorkerJobInput {
  body: FailBody;
}

export type FailResult =
  | { outcome: "requeued" | "failed" | "canceled"; workerPaused: boolean }
  | "lease_lost"
  | "invalid";

// The worker reports a failure; the portal decides between retry, failure and cancel.
export async function failJob(input: FailInput): Promise<FailResult> {
  const { worker, jobId, attempt, body, now } = input;
  const result = await transact(async (tx) => {
    const [job] = await tx
      .select()
      .from(requests)
      .where(
        and(
          eq(requests.id, jobId),
          eq(requests.workerId, worker.id),
          eq(requests.attempt, attempt),
          inArray(requests.status, [...LEASED_STATUSES]),
        ),
      );
    const [current] = await tx.select().from(workers).where(eq(workers.id, worker.id));
    if (!job || !current) return "lease_lost";
    return applyFailure(tx, {
      job,
      worker: current,
      code: body.code,
      message: body.message,
      detail: body.detail,
      machineSeconds: body.machineSeconds,
      now,
    });
  });
  if (typeof result === "string") return result;
  await removeFiles(result.removedPaths);
  invalidateQueueSnapshot();
  return { outcome: result.outcome, workerPaused: result.workerPaused };
}

export interface CancelInput {
  jobId: string;
  by: CancelActor;
  actorUserId: string;
  reason?: string | null;
  now: number;
}

export type CancelResult = { status: string; cancelRequested: boolean } | "not_found" | "invalid_state";

interface CancelOutcome {
  result: CancelResult;
  removed: string[];
  // The demo of a job that will never run, to delete unless another job uses it.
  releasedDemo: string | null;
}

// Cancels at once when no worker holds the job; otherwise asks the worker to stop.
export async function cancelJob(input: CancelInput): Promise<CancelResult> {
  const { jobId, by, now } = input;
  const actor = `${by}:${input.actorUserId}`;

  const result = await transact(async (tx): Promise<CancelOutcome> => {
    const [job] = await tx.select().from(requests).where(eq(requests.id, jobId));
    if (!job) return { result: "not_found", removed: [], releasedDemo: null };
    const event = {
      actor,
      at: now,
      requestId: job.id,
      workerId: job.workerId,
      subjectUserId: job.userId,
      detail: input.reason ?? null,
    };

    if (job.status === "running" || job.status === "uploading") {
      if (job.cancelRequestedAt === null) {
        await tx
          .update(requests)
          .set({ cancelRequestedAt: now, canceledBy: by, updatedAt: new Date(now) })
          .where(eq(requests.id, job.id));
        await recordEvent({ ...event, type: "cancel_requested" }, tx);
      }
      return {
        result: { status: job.status, cancelRequested: true },
        removed: [],
        releasedDemo: null,
      };
    }

    const legacy = by === "admin" && isLegacyStatus(job.status) && job.status !== "rejected";
    if (job.status !== "awaiting_demo" && job.status !== "queued" && !legacy) {
      return { result: "invalid_state", removed: [], releasedDemo: null };
    }
    await tx
      .update(requests)
      .set({
        status: "canceled",
        finishedAt: now,
        canceledBy: by,
        priorityBoost: 0,
        leaseExpiresAt: null,
        updatedAt: new Date(now),
      })
      .where(eq(requests.id, job.id));
    const removed = await tx
      .delete(requestArtifacts)
      .where(eq(requestArtifacts.requestId, job.id))
      .returning({ path: requestArtifacts.path });
    await recordEvent({ ...event, type: "canceled" }, tx);
    // A job its user cancels before it ever ran will not be retried, so its demo can go now.
    const neverRan = by === "user" && job.attempt === 0 && isCloudKind(job.kind);
    return {
      result: { status: "canceled", cancelRequested: false },
      removed: removed.map((row) => row.path),
      releasedDemo: neverRan ? job.demoPath : null,
    };
  });
  await removeFiles(result.removed);
  if (result.releasedDemo !== null) await releaseDemo(result.releasedDemo, now);
  invalidateQueueSnapshot();
  return result.result;
}

export interface AdminJobInput {
  jobId: string;
  adminUserId: string;
  now: number;
}

// Requeues a failed or cancelled job with a fresh attempt count and its original seniority.
export async function retryJob(
  input: AdminJobInput,
): Promise<"ok" | "not_found" | "invalid_state" | "demo_gone"> {
  const { jobId, now } = input;
  const result = await withDemoLock(async () => {
    const [job] = await db.select().from(requests).where(eq(requests.id, jobId));
    if (!job) return "not_found";
    const retryable = job.status === "failed" || job.status === "canceled";
    if (!retryable || !isCloudKind(job.kind)) return "invalid_state";
    if (!job.demoPath || (await demoFileSize(job.demoPath)) === null) return "demo_gone";

    return transact(async (tx) => {
      const updated = await tx
        .update(requests)
        .set({
          status: "queued",
          attempt: 0,
          machineRequeues: 0,
          // A boost was for the run that ended; the operator can give a new one.
          priorityBoost: 0,
          enqueuedAt: job.enqueuedAt ?? now,
          machineSeconds: 0,
          workerId: null,
          leaseExpiresAt: null,
          claimedAt: null,
          uploadingAt: null,
          localJobId: null,
          stage: null,
          progressPercent: null,
          progressDetail: null,
          finishedAt: null,
          failureCode: null,
          failureReason: null,
          failureDetail: null,
          cancelRequestedAt: null,
          canceledBy: null,
          updatedAt: new Date(now),
        })
        .where(and(eq(requests.id, jobId), eq(requests.status, job.status)))
        .returning({ id: requests.id });
      if (updated.length === 0) return "invalid_state";
      await recordEvent(
        {
          type: "retried",
          actor: `admin:${input.adminUserId}`,
          at: now,
          requestId: jobId,
          subjectUserId: job.userId,
        },
        tx,
      );
      return "ok";
    });
  });
  invalidateQueueSnapshot();
  return result;
}

export interface PriorityInput extends AdminJobInput {
  action: "front" | "reset";
}

// "front" puts the job ahead of every other boosted job; "reset" removes the boost.
export async function setPriority(
  input: PriorityInput,
): Promise<"ok" | "not_found" | "invalid_state"> {
  const { jobId, action, now } = input;
  const result = await transact(async (tx) => {
    const [job] = await tx.select().from(requests).where(eq(requests.id, jobId));
    if (!job) return "not_found";
    if (job.status !== "queued") return "invalid_state";
    const [top] = await tx
      .select({ boost: sql<number>`coalesce(max(${requests.priorityBoost}), 0)` })
      .from(requests)
      .where(eq(requests.status, "queued"));
    const priorityBoost = action === "front" ? Number(top?.boost ?? 0) + 1 : 0;
    await tx
      .update(requests)
      .set({ priorityBoost, updatedAt: new Date(now) })
      .where(eq(requests.id, jobId));
    await recordEvent(
      {
        type: "boosted",
        actor: `admin:${input.adminUserId}`,
        at: now,
        requestId: jobId,
        subjectUserId: job.userId,
        detail: action,
      },
      tx,
    );
    return "ok";
  });
  invalidateQueueSnapshot();
  return result;
}

let leaseSweeperStarted = false;

// Started on server boot, next to the retention sweeper.
export function startLeaseSweeper(): void {
  if (leaseSweeperStarted) return;
  leaseSweeperStarted = true;
  const timer = setInterval(() => {
    sweepLeases(Date.now()).catch((err: unknown) => {
      console.error("lease sweep failed:", err);
    });
  }, LEASE_SWEEP_INTERVAL_MS);
  timer.unref();
}
