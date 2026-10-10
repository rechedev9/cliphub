import { and, desc, eq, gte, inArray, isNotNull, isNull, lt, ne, sql } from "drizzle-orm";

import { db } from "../db/client.ts";
import {
  devices,
  events,
  requestArtifacts,
  requests,
  userCloud,
  users,
  workers,
} from "../db/schema.ts";
import { adminUserIds } from "./admin.ts";
import { artifactExpiresAt } from "./artifact-parts.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import { readStoredSpec } from "./job-spec.ts";
import { userCloudState, userUsage } from "./job-store.ts";
import {
  ACTIVE_STATUSES,
  LEASED_STATUSES,
  isArtifactKind,
  isFailureCode,
  isLeasedStatus,
  isLegacyStatus,
  isPauseSource,
  isStage,
  isWorkerBlockCode,
  isWorkerState,
  type ArtifactKind,
  type CloudAccess,
  type CloudKind,
  type FailureCode,
  type PauseSource,
  type QueueState,
  type Stage,
  type WorkerBlockCode,
  type WorkerState,
} from "./job-types.ts";
import { isWorkerOnline, usageByUser } from "./queue-policy.ts";
import {
  estimateWaitSeconds,
  queueSnapshot,
  type JobRow,
  type QueueSnapshot,
  type WorkerRow,
} from "./queue-store.ts";
import { storageStats } from "./storage-guard.ts";
import {
  effectiveLimits,
  loadAccessDefault,
  loadUserLimits,
  resolveAccess,
  type LimitOverrides,
  type UserLimits,
} from "./user-limits.ts";
import { parseWorkerHealth, type WorkerHealth } from "./worker-protocol.ts";

type ArtifactRow = typeof requestArtifacts.$inferSelect;
type EventRow = typeof events.$inferSelect;

const DAY_MS = 24 * 60 * 60 * 1000;
const MAX_USERS = 1000;
const MAX_USER_JOB_ROWS = 20000;
const DEFAULT_PAGE = 50;
const MAX_PAGE = 200;

export interface StudioArtifact {
  id: string;
  kind: ArtifactKind;
  variant: string;
  name: string;
  sizeBytes: number;
  sha256: string;
  expiresAt: number;
}

export interface StudioJob {
  id: string;
  kind: string;
  title: string;
  status: string;
  stage: Stage | null;
  progressPercent: number | null;
  createdAt: number;
  enqueuedAt: number | null;
  startedAt: number | null;
  finishedAt: number | null;
  estimatedSeconds: number | null;
  attempt: number;
  cancelRequested: boolean;
  queue: {
    position: number;
    estimatedStartAt: number | null;
    estimatedDoneAt: number | null;
    state: QueueState;
  } | null;
  failure: { code: FailureCode; message: string } | null;
  artifacts: StudioArtifact[];
}

export interface AdminJob extends StudioJob {
  user: { id: string; name: string | null; email: string | null };
  // What the worker last said it was doing, for example "REC 2/5".
  progressDetail: string | null;
  workerId: string | null;
  localJobId: string | null;
  leaseExpiresAt: number | null;
  priorityBoost: number;
  machineSeconds: number;
  maxAttempts: number;
  waitedSeconds: number;
  failureDetail: string | null;
  canceledBy: string | null;
  demo: { fileName: string; sizeBytes: number; sha256: string; present: boolean };
  legacyStatus: boolean;
}

export interface AdminWorker {
  id: string;
  name: string;
  online: boolean;
  lastSeenAt: number | null;
  state: WorkerState | null;
  blocked: { code: WorkerBlockCode; detail: string } | null;
  paused: boolean;
  pausedBy: PauseSource | null;
  pauseReason: string | null;
  revoked: boolean;
  createdAt: number;
  health: WorkerHealth | null;
  currentJobId: string | null;
}

export interface AdminUser {
  id: string;
  name: string | null;
  email: string | null;
  image: string | null;
  access: CloudAccess;
  isAdmin: boolean;
  limits: UserLimits;
  overrides: LimitOverrides;
  usage: { active: number; secondsLast24h: number; jobs7d: number; failed7d: number };
  devices: number;
  lastJobAt: number | null;
  note: string | null;
}

export interface AdminEvent {
  id: string;
  at: number;
  actor: string;
  type: string;
  detail: string | null;
}

function studioArtifact(artifact: ArtifactRow, retentionDays: number): StudioArtifact {
  return {
    id: artifact.id,
    kind: isArtifactKind(artifact.kind) ? artifact.kind : "video",
    variant: artifact.variant,
    name: artifact.name,
    sizeBytes: artifact.sizeBytes,
    sha256: artifact.sha256 ?? "",
    expiresAt: artifactExpiresAt(artifact, retentionDays),
  };
}

interface JobViewContext {
  snapshot: QueueSnapshot;
  artifacts: Map<string, ArtifactRow[]>;
  retentionDays: number;
}

function studioJob(job: JobRow, context: JobViewContext): StudioJob {
  const { snapshot } = context;
  const placement = snapshot.placements.get(job.id);
  const inProgress = job.status === "running" || job.status === "uploading";
  const failed = job.status === "failed" || job.status === "rejected";
  return {
    id: job.id,
    kind: job.kind,
    title: job.title ?? "",
    status: job.status,
    stage: job.status === "running" && isStage(job.stage) ? job.stage : null,
    progressPercent: inProgress ? job.progressPercent : null,
    createdAt: job.createdAt.getTime(),
    enqueuedAt: job.enqueuedAt,
    startedAt: job.claimedAt?.getTime() ?? null,
    finishedAt: job.finishedAt,
    estimatedSeconds: job.estimatedSeconds,
    attempt: job.attempt,
    cancelRequested: job.cancelRequestedAt !== null,
    queue:
      job.status === "queued"
        ? {
            position: placement?.position ?? snapshot.queuedCount + 1,
            estimatedStartAt: placement?.estimatedStartAt ?? null,
            estimatedDoneAt: placement?.estimatedDoneAt ?? null,
            state: snapshot.state,
          }
        : null,
    failure: failed
      ? {
          code: isFailureCode(job.failureCode) ? job.failureCode : "job_failed",
          message: job.failureReason ?? "",
        }
      : null,
    artifacts: (context.artifacts.get(job.id) ?? [])
      .filter((artifact) => artifact.status === "ready")
      .map((artifact) => studioArtifact(artifact, context.retentionDays)),
  };
}

async function jobViewContext(jobs: JobRow[], now: number): Promise<JobViewContext> {
  const ids = jobs.map((job) => job.id);
  const rows =
    ids.length === 0
      ? []
      : await db
          .select()
          .from(requestArtifacts)
          .where(inArray(requestArtifacts.requestId, ids))
          .orderBy(requestArtifacts.variant, requestArtifacts.name);
  const artifacts = new Map<string, ArtifactRow[]>();
  for (const row of rows) {
    artifacts.set(row.requestId, [...(artifacts.get(row.requestId) ?? []), row]);
  }
  return {
    snapshot: await queueSnapshot(now),
    artifacts,
    retentionDays: loadCloudConfig().artifactRetentionDays,
  };
}

// The one place that turns job rows into what Studio and the dashboard see.
export async function studioJobs(jobs: JobRow[], now: number): Promise<StudioJob[]> {
  const context = await jobViewContext(jobs, now);
  return jobs.map((job) => studioJob(job, context));
}

function waitedSeconds(job: JobRow, now: number): number {
  if (job.enqueuedAt === null) return 0;
  const until = job.status === "queued" ? now : (job.claimedAt?.getTime() ?? job.enqueuedAt);
  return Math.max(0, Math.round((until - job.enqueuedAt) / 1000));
}

async function adminJobs(jobs: JobRow[], now: number): Promise<AdminJob[]> {
  const context = await jobViewContext(jobs, now);
  const userIds = [...new Set(jobs.map((job) => job.userId))];
  const userRows =
    userIds.length === 0
      ? []
      : await db
          .select({ id: users.id, name: users.name, email: users.email })
          .from(users)
          .where(inArray(users.id, userIds));
  const usersById = new Map(userRows.map((user) => [user.id, user]));

  return jobs.map((job) => ({
    ...studioJob(job, context),
    user: usersById.get(job.userId) ?? { id: job.userId, name: null, email: null },
    progressDetail: isLeasedStatus(job.status) ? job.progressDetail : null,
    workerId: job.workerId,
    localJobId: job.localJobId,
    leaseExpiresAt: job.leaseExpiresAt,
    priorityBoost: job.priorityBoost,
    machineSeconds: job.machineSeconds,
    maxAttempts: job.maxAttempts,
    waitedSeconds: waitedSeconds(job, now),
    failureDetail: job.failureDetail,
    canceledBy: job.canceledBy,
    demo: {
      fileName: job.demoOriginalName ?? "demo.dem",
      sizeBytes: job.demoSizeBytes ?? 0,
      sha256: job.demoSha256 ?? "",
      present: job.demoPath !== null,
    },
    legacyStatus: isLegacyStatus(job.status),
  }));
}

function storedHealth(raw: string | null): WorkerHealth | null {
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return parseWorkerHealth(parsed);
  } catch {
    return null;
  }
}

function adminWorker(
  worker: WorkerRow,
  options: { currentJobId: string | null; now: number },
): AdminWorker {
  return {
    id: worker.id,
    name: worker.name,
    online: isWorkerOnline(worker, options.now),
    lastSeenAt: worker.lastSeenAt,
    state: isWorkerState(worker.state) ? worker.state : null,
    blocked: isWorkerBlockCode(worker.blockedCode)
      ? { code: worker.blockedCode, detail: worker.blockedDetail ?? "" }
      : null,
    paused: worker.paused,
    pausedBy: isPauseSource(worker.pausedBy) ? worker.pausedBy : null,
    pauseReason: worker.pauseReason,
    revoked: worker.revokedAt !== null,
    createdAt: worker.createdAt,
    health: storedHealth(worker.health),
    currentJobId: options.currentJobId,
  };
}

export async function adminWorkers(now: number, workerId?: string): Promise<AdminWorker[]> {
  const rows = await db
    .select()
    .from(workers)
    .where(workerId ? eq(workers.id, workerId) : undefined)
    .orderBy(workers.createdAt);
  const running = await db
    .select({ id: requests.id, workerId: requests.workerId })
    .from(requests)
    .where(eq(requests.status, "running"));
  const currentJob = new Map(running.map((job) => [job.workerId, job.id]));
  return rows.map((worker) =>
    adminWorker(worker, { currentJobId: currentJob.get(worker.id) ?? null, now }),
  );
}

function median(values: number[]): number | null {
  if (values.length === 0) return null;
  const sorted = [...values].sort((a, b) => a - b);
  const mid = Math.floor(sorted.length / 2);
  const upper = sorted[mid] ?? 0;
  return sorted.length % 2 === 1 ? upper : ((sorted[mid - 1] ?? upper) + upper) / 2;
}

async function storedBytes(): Promise<{ demoBytes: number; artifactBytes: number }> {
  const demos = await db
    .selectDistinct({ path: requests.demoPath, sizeBytes: requests.demoSizeBytes })
    .from(requests)
    .where(isNotNull(requests.demoPath));
  const [artifacts] = await db
    .select({ total: sql<number>`coalesce(sum(${requestArtifacts.sizeBytes}), 0)` })
    .from(requestArtifacts);
  return {
    demoBytes: demos.reduce((sum, demo) => sum + (demo.sizeBytes ?? 0), 0),
    artifactBytes: Number(artifacts?.total ?? 0),
  };
}

async function countPendingUsers(): Promise<number> {
  const accessDefault = loadAccessDefault();
  const admins = await adminUserIds();
  const rows = await db
    .select({ id: users.id, access: userCloud.access })
    .from(users)
    .leftJoin(userCloud, eq(userCloud.userId, users.id));
  return rows.filter(
    (row) =>
      resolveAccess({ stored: row.access, isAdmin: admins.has(row.id), accessDefault }) ===
      "pending",
  ).length;
}

export async function adminOverview(now: number) {
  const snapshot = await queueSnapshot(now);
  const active = await db
    .select()
    .from(requests)
    .where(inArray(requests.status, ["queued", ...LEASED_STATUSES]));
  const jobs = await adminJobs(active, now);
  const position = (job: AdminJob) => job.queue?.position ?? 0;

  const dayAgo = now - DAY_MS;
  const finished = await db
    .select({ status: requests.status, machineSeconds: requests.machineSeconds })
    .from(requests)
    .where(gte(requests.finishedAt, dayAgo));
  const started = await db
    .select({ enqueuedAt: requests.enqueuedAt, claimedAt: requests.claimedAt })
    .from(requests)
    .where(and(gte(requests.claimedAt, new Date(dayAgo)), isNotNull(requests.enqueuedAt)));
  const waits = started.map((job) =>
    Math.max(0, ((job.claimedAt?.getTime() ?? 0) - (job.enqueuedAt ?? 0)) / 1000),
  );
  const medianWait = median(waits);
  const countOf = (status: string) => finished.filter((job) => job.status === status).length;

  return {
    now,
    workers: (await adminWorkers(now)).filter((worker) => !worker.revoked),
    running: jobs.filter((job) => job.status === "running"),
    uploading: jobs.filter((job) => job.status === "uploading"),
    queue: jobs
      .filter((job) => job.status === "queued")
      .sort((a, b) => position(a) - position(b)),
    stats: {
      queued: snapshot.queuedCount,
      done24h: countOf("done"),
      failed24h: countOf("failed"),
      canceled24h: countOf("canceled"),
      medianWaitSeconds24h: medianWait === null ? null : Math.round(medianWait),
      machineSeconds24h: finished.reduce((sum, job) => sum + job.machineSeconds, 0),
      storage: { ...(await storageStats()), ...(await storedBytes()) },
    },
    pendingUsers: await countPendingUsers(),
  };
}

function countBy(rows: { userId: string }[]): Map<string, number> {
  const counts = new Map<string, number>();
  for (const row of rows) counts.set(row.userId, (counts.get(row.userId) ?? 0) + 1);
  return counts;
}

export interface AdminUserFilter {
  query: string;
  access: CloudAccess | null;
  now: number;
}

export async function adminUsers(filter: AdminUserFilter): Promise<AdminUser[]> {
  const { now } = filter;
  const defaults = loadUserLimits();
  const accessDefault = loadAccessDefault();
  const admins = await adminUserIds();

  const rows = await db
    .select({ user: users, cloud: userCloud })
    .from(users)
    .leftJoin(userCloud, eq(userCloud.userId, users.id))
    .limit(MAX_USERS);
  const cloudJobs = await db
    .select({
      userId: requests.userId,
      status: requests.status,
      createdAt: requests.createdAt,
      finishedAt: requests.finishedAt,
      machineSeconds: requests.machineSeconds,
      estimatedSeconds: requests.estimatedSeconds,
      failureCode: requests.failureCode,
      canceledBy: requests.canceledBy,
    })
    .from(requests)
    .where(ne(requests.kind, "manual"))
    .orderBy(desc(requests.createdAt))
    .limit(MAX_USER_JOB_ROWS);
  const deviceRows = await db
    .select({ userId: devices.userId })
    .from(devices)
    .where(isNull(devices.revokedAt));

  const weekAgo = now - 7 * DAY_MS;
  const recent = cloudJobs.filter((job) => job.createdAt.getTime() >= weekAgo);
  const usage = usageByUser(cloudJobs, now);
  const activeCount = countBy(
    cloudJobs.filter((job) => ACTIVE_STATUSES.some((status) => status === job.status)),
  );
  const jobs7d = countBy(recent);
  const failed7d = countBy(recent.filter((job) => job.status === "failed"));
  const deviceCount = countBy(deviceRows);
  const lastJobAt = new Map<string, number>();
  for (const job of cloudJobs) {
    if (!lastJobAt.has(job.userId)) lastJobAt.set(job.userId, job.createdAt.getTime());
  }

  const needle = filter.query.trim().toLowerCase();
  const views = rows.map(({ user, cloud }): AdminUser => {
    const overrides = {
      maxActive: cloud?.maxActive ?? null,
      dailySeconds: cloud?.dailySeconds ?? null,
    };
    return {
      id: user.id,
      name: user.name,
      email: user.email,
      image: user.image,
      access: resolveAccess({
        stored: cloud?.access ?? null,
        isAdmin: admins.has(user.id),
        accessDefault,
      }),
      isAdmin: admins.has(user.id),
      limits: effectiveLimits(defaults, overrides),
      overrides,
      usage: {
        active: activeCount.get(user.id) ?? 0,
        secondsLast24h: usage.get(user.id) ?? 0,
        jobs7d: jobs7d.get(user.id) ?? 0,
        failed7d: failed7d.get(user.id) ?? 0,
      },
      devices: deviceCount.get(user.id) ?? 0,
      lastJobAt: lastJobAt.get(user.id) ?? null,
      note: cloud?.note ?? null,
    };
  });
  return views
    .filter((view) => filter.access === null || view.access === filter.access)
    .filter(
      (view) =>
        needle === "" ||
        (view.name ?? "").toLowerCase().includes(needle) ||
        (view.email ?? "").toLowerCase().includes(needle),
    )
    .sort((a, b) => (b.lastJobAt ?? 0) - (a.lastJobAt ?? 0));
}

function adminEvent(event: EventRow): AdminEvent {
  return {
    id: event.id,
    at: event.at,
    actor: event.actor,
    type: event.type,
    detail: event.detail,
  };
}

export async function adminJobDetail(jobId: string, now: number) {
  const [row] = await db.select().from(requests).where(eq(requests.id, jobId));
  if (!row) return null;
  const [job] = await adminJobs([row], now);
  if (!job) return null;
  const timeline = await db
    .select()
    .from(events)
    .where(eq(events.requestId, jobId))
    .orderBy(events.at);
  return { job, spec: readStoredSpec(row.spec), events: timeline.map(adminEvent) };
}

export interface AdminJobFilter {
  status: string | null;
  kind: string | null;
  userId: string | null;
  // Only jobs updated before this time (ms), for paging.
  before: number | null;
  limit: number | null;
  now: number;
}

// History for the panel, newest change first. nextBefore is the cursor of the next page.
export async function adminJobPage(filter: AdminJobFilter) {
  const limit = Math.min(MAX_PAGE, Math.max(1, Math.floor(filter.limit ?? DEFAULT_PAGE)));
  const rows = await db
    .select()
    .from(requests)
    .where(
      and(
        filter.status ? eq(requests.status, filter.status) : undefined,
        filter.kind ? eq(requests.kind, filter.kind) : undefined,
        filter.userId ? eq(requests.userId, filter.userId) : undefined,
        filter.before === null ? undefined : lt(requests.updatedAt, new Date(filter.before)),
      ),
    )
    .orderBy(desc(requests.updatedAt))
    .limit(limit);
  const last = rows[rows.length - 1];
  return {
    jobs: await adminJobs(rows, filter.now),
    nextBefore: rows.length === limit && last ? last.updatedAt.getTime() : null,
  };
}

export async function adminEventLog(limit: number | null) {
  const rows = await db
    .select()
    .from(events)
    .orderBy(desc(events.at))
    .limit(Math.min(MAX_PAGE * 5, Math.max(1, Math.floor(limit ?? 100))));
  return rows.map((event) => ({
    ...adminEvent(event),
    requestId: event.requestId,
    workerId: event.workerId,
    subjectUserId: event.subjectUserId,
  }));
}

// GET /api/studio/me: who the caller is, what they may do and how busy the cloud is.
export async function studioMe(userId: string, now: number) {
  const config = loadCloudConfig();
  const [user] = await db
    .select({ id: users.id, name: users.name, email: users.email, image: users.image })
    .from(users)
    .where(eq(users.id, userId));
  if (!user) return null;
  const state = await userCloudState(userId);
  const usage = await userUsage(db, { userId, now });
  const snapshot = await queueSnapshot(now);
  const waitSeconds: Partial<Record<CloudKind, number | null>> = {};
  for (const kind of config.kinds) {
    waitSeconds[kind] = estimateWaitSeconds({ snapshot, userId, kind, now });
  }
  return {
    user,
    access: state.access,
    limits: { ...state.limits, maxDemoBytes: config.maxDemoBytes },
    usage: {
      active: usage.active,
      secondsLast24h: usage.secondsLast24h,
      secondsCommitted: usage.secondsCommitted,
    },
    kinds: config.kinds,
    queue: { state: snapshot.state, queued: snapshot.queuedCount, waitSeconds },
  };
}
