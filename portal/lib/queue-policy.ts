import {
  isRetryCode,
  isTerminalCode,
  type CloudKind,
  type QueueState,
} from "./job-types.ts";

const DAY_MS = 24 * 60 * 60 * 1000;
const OVERDUE_MS = 10800 * 1000;
const FAIRNESS_SECONDS = 1800;
const CAPTURE_FACTOR = 1.3;
const BASE_SECONDS: Record<CloudKind, number> = { short: 240, full_demo: 420 };
const RENDER_FACTOR: Record<CloudKind, number> = { short: 1.5, full_demo: 1.6 };
const CALIBRATION_WINDOW = 30;
const CALIBRATION_MIN_SAMPLES = 5;
const MIN_REMAINING_SECONDS = 60;

function clamp(value: number, range: { min: number; max: number }): number {
  return Math.min(range.max, Math.max(range.min, value));
}

export interface EstimateInput {
  kind: CloudKind;
  captureSeconds: number;
  calibration: number;
}

export function estimateSeconds(input: EstimateInput): number {
  const { kind, captureSeconds, calibration: factor } = input;
  const raw =
    BASE_SECONDS[kind] +
    CAPTURE_FACTOR * captureSeconds +
    RENDER_FACTOR[kind] * captureSeconds;
  return Math.round(raw * factor);
}

export function maxRuntimeSeconds(estimatedSeconds: number): number {
  return clamp(3 * estimatedSeconds, { min: 1200, max: 14400 });
}

export interface CalibrationSample {
  machineSeconds: number;
  // The estimate before calibration, so the factor does not feed on itself.
  uncalibratedSeconds: number;
}

// Median of actual over estimated for the latest finished jobs, newest first.
export function calibration(samples: CalibrationSample[]): number {
  const ratios = samples
    .filter((s) => s.machineSeconds > 0 && s.uncalibratedSeconds > 0)
    .slice(0, CALIBRATION_WINDOW)
    .map((s) => s.machineSeconds / s.uncalibratedSeconds)
    .sort((a, b) => a - b);
  if (ratios.length < CALIBRATION_MIN_SAMPLES) return 1;
  const mid = Math.floor(ratios.length / 2);
  const upper = ratios[mid] ?? 1;
  const lower = ratios[mid - 1] ?? upper;
  const median = ratios.length % 2 === 1 ? upper : (lower + upper) / 2;
  return clamp(median, { min: 0.5, max: 3 });
}

export interface UsageJob {
  userId: string;
  status: string;
  finishedAt: number | null;
  machineSeconds: number;
  estimatedSeconds: number | null;
  failureCode: string | null;
  canceledBy: string | null;
}

function usageOf(job: UsageJob, now: number): number {
  // machineSeconds of a running job is what its earlier attempts used; this one is worth its estimate.
  if (job.status === "running") return job.machineSeconds + (job.estimatedSeconds ?? 0);
  if (job.status === "uploading") return job.machineSeconds;
  if (job.finishedAt === null || job.finishedAt < now - DAY_MS) return 0;
  const counts =
    job.status === "done" ||
    (job.status === "failed" &&
      (isRetryCode(job.failureCode) || isTerminalCode(job.failureCode))) ||
    (job.status === "canceled" && job.canceledBy === "user");
  return counts ? job.machineSeconds : 0;
}

// Machine seconds one user consumed in the last 24 hours, from that user's jobs.
export function usage24h(jobs: UsageJob[], now: number): number {
  return jobs.reduce((sum, job) => sum + usageOf(job, now), 0);
}

export function usageByUser(jobs: UsageJob[], now: number): Map<string, number> {
  const usage = new Map<string, number>();
  for (const job of jobs) {
    const seconds = usageOf(job, now);
    if (seconds > 0) usage.set(job.userId, (usage.get(job.userId) ?? 0) + seconds);
  }
  return usage;
}

export interface QueueJob {
  id: string;
  userId: string;
  kind: CloudKind;
  enqueuedAt: number;
  estimatedSeconds: number;
  priorityBoost: number;
}

function bySeniority(a: QueueJob, b: QueueJob): number {
  return a.enqueuedAt - b.enqueuedAt || (a.id < b.id ? -1 : a.id > b.id ? 1 : 0);
}

export interface CandidateInput {
  queued: QueueJob[];
  kinds: readonly CloudKind[];
  blockedUserIds: ReadonlySet<string>;
}

// Each user's oldest eligible job, plus every job an operator moved to the front.
export function selectCandidates(input: CandidateInput): QueueJob[] {
  const eligible = input.queued.filter(
    (job) => input.kinds.includes(job.kind) && !input.blockedUserIds.has(job.userId),
  );
  const oldest = new Map<string, QueueJob>();
  for (const job of eligible) {
    const current = oldest.get(job.userId);
    if (!current || bySeniority(job, current) < 0) oldest.set(job.userId, job);
  }
  return eligible.filter(
    (job) => job.priorityBoost > 0 || oldest.get(job.userId) === job,
  );
}

export interface PickInput {
  candidates: QueueJob[];
  usageByUser: ReadonlyMap<string, number>;
  now: number;
}

function score(job: QueueJob, input: PickInput): number {
  const weight = clamp(job.estimatedSeconds, { min: 300, max: 1800 });
  const waited = Math.max(0, (input.now - job.enqueuedAt) / 1000);
  const usage = input.usageByUser.get(job.userId) ?? 0;
  return (waited + weight) / weight / (1 + usage / FAIRNESS_SECONDS);
}

export function pickNext(input: PickInput): QueueJob | null {
  const { candidates, now } = input;
  const boosted = candidates.filter((job) => job.priorityBoost > 0);
  if (boosted.length > 0) {
    return (
      boosted.sort(
        (a, b) => b.priorityBoost - a.priorityBoost || bySeniority(a, b),
      )[0] ?? null
    );
  }
  const overdue = candidates.filter((job) => now - job.enqueuedAt >= OVERDUE_MS);
  if (overdue.length > 0) return overdue.sort(bySeniority)[0] ?? null;
  const scored = candidates.map((job) => ({ job, score: score(job, input) }));
  scored.sort((a, b) => b.score - a.score || bySeniority(a.job, b.job));
  return scored[0]?.job ?? null;
}

export interface RunningJob {
  estimatedSeconds: number;
  claimedAt: number;
}

export interface SimulateInput {
  queued: QueueJob[];
  running: RunningJob[];
  usageByUser: ReadonlyMap<string, number>;
  now: number;
  queueState: QueueState;
}

export interface QueuePlacement {
  position: number;
  estimatedStartAt: number | null;
  estimatedDoneAt: number | null;
}

const ALL_KINDS: readonly CloudKind[] = ["short", "full_demo"];
const NO_USERS: ReadonlySet<string> = new Set();

// Replays pickNext on a virtual clock. Map iteration order is the order jobs will start.
export function simulateQueue(input: SimulateInput): Map<string, QueuePlacement> {
  const { now, queueState } = input;
  const online = queueState === "online";
  const usage = new Map(input.usageByUser);
  let clock = now;
  for (const job of input.running) {
    const elapsed = (now - job.claimedAt) / 1000;
    const remaining = Math.max(MIN_REMAINING_SECONDS, job.estimatedSeconds - elapsed);
    clock = Math.max(clock, now + remaining * 1000);
  }

  const placements = new Map<string, QueuePlacement>();
  let remaining = [...input.queued];
  while (remaining.length > 0) {
    const candidates = selectCandidates({
      queued: remaining,
      kinds: ALL_KINDS,
      blockedUserIds: NO_USERS,
    });
    const next = pickNext({ candidates, usageByUser: usage, now: clock });
    if (!next) break;
    const startAt = clock;
    clock += next.estimatedSeconds * 1000;
    placements.set(next.id, {
      position: placements.size + 1,
      estimatedStartAt: online ? startAt : null,
      estimatedDoneAt: online ? clock : null,
    });
    usage.set(next.userId, (usage.get(next.userId) ?? 0) + next.estimatedSeconds);
    remaining = remaining.filter((job) => job.id !== next.id);
  }
  return placements;
}

export interface WorkerPresence {
  lastSeenAt: number | null;
  revokedAt: number | null;
  paused: boolean;
  state: string | null;
}

const WORKER_ONLINE_MS = 60 * 1000;

export function isWorkerOnline(worker: WorkerPresence, now: number): boolean {
  return (
    worker.revokedAt === null &&
    worker.lastSeenAt !== null &&
    now - worker.lastSeenAt <= WORKER_ONLINE_MS
  );
}

// offline: nobody has been seen for a minute. paused: someone is there but cannot take work.
export function queueStateOf(workers: WorkerPresence[], now: number): QueueState {
  const online = workers.filter((worker) => isWorkerOnline(worker, now));
  if (online.length === 0) return "offline";
  const ready = online.some((worker) => !worker.paused && worker.state !== "blocked");
  return ready ? "online" : "paused";
}
