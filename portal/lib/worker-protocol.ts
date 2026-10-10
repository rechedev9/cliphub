import { ARTIFACT_VARIANT_PATTERN, isArtifactName } from "./artifact-parts.ts";
import {
  isArtifactKind,
  isCloudKind,
  isFailureCode,
  isRecord,
  isStage,
  isWorkerBlockCode,
  isWorkerState,
  type ArtifactKind,
  type CloudKind,
  type FailureCode,
  type Stage,
  type WorkerBlockCode,
  type WorkerState,
} from "./job-types.ts";
import { isSha256Hex } from "./tokens.ts";

// Far more than a worker can hold; entries past it are ignored, never a reason to refuse.
const MAX_HEARTBEAT_JOBS = 200;
export const ATTEMPT_HEADER = "x-cliphub-attempt";
const MAX_MESSAGE_CHARS = 500;
const MAX_DETAIL_CHARS = 4000;
const MAX_PROGRESS_DETAIL_CHARS = 200;
const MAX_HEALTH_STRING_CHARS = 120;
const MAX_ID_CHARS = 64;

export interface WorkerHealth {
  hostname: string;
  studioVersion: string;
  cs2PatchVersion: string;
  hlaeVersion: string;
  steamRunning: boolean;
  recordEnabled: boolean;
  diskFreeBytes: number;
  diskTotalBytes: number;
  kinds: CloudKind[];
  uptimeSeconds: number;
  // The kill plan schema this worker's Studio produces; empty when unknown.
  planSchema: string;
}

export interface WorkerBlock {
  code: WorkerBlockCode;
  detail: string;
}

export interface HeartbeatJob {
  id: string;
  phase: "running" | "uploading";
  // The attempt the worker is working on; null when it did not say.
  attempt: number | null;
  stage: Stage | null;
  percent: number | null;
  detail: string | null;
  localJobId: string | null;
}

export interface HeartbeatBody {
  state: WorkerState;
  blocked: WorkerBlock | null;
  health: WorkerHealth;
  jobs: HeartbeatJob[];
}

function text(value: unknown, max: number): string {
  return typeof value === "string" ? value.slice(0, max) : "";
}

function optionalText(value: unknown, max: number): string | null {
  return typeof value === "string" && value.length > 0 ? value.slice(0, max) : null;
}

function count(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0
    ? Math.floor(value)
    : 0;
}

function positiveInt(value: unknown): number | null {
  return typeof value === "number" && Number.isSafeInteger(value) && value > 0 ? value : null;
}

// The fencing token of a job route: the attempt number the claim returned.
export function parseAttemptHeader(request: Request): number | null {
  const raw = request.headers.get(ATTEMPT_HEADER);
  if (raw === null || !/^[1-9]\d{0,8}$/.test(raw)) return null;
  return Number(raw);
}

function percent(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isFinite(value)) return null;
  return Math.min(100, Math.max(0, Math.round(value)));
}

// Health is informational, so unknown or missing fields become empty values.
export function parseWorkerHealth(input: unknown): WorkerHealth {
  const health = isRecord(input) ? input : {};
  return {
    hostname: text(health.hostname, MAX_HEALTH_STRING_CHARS),
    studioVersion: text(health.studioVersion, MAX_HEALTH_STRING_CHARS),
    cs2PatchVersion: text(health.cs2PatchVersion, MAX_HEALTH_STRING_CHARS),
    hlaeVersion: text(health.hlaeVersion, MAX_HEALTH_STRING_CHARS),
    steamRunning: health.steamRunning === true,
    recordEnabled: health.recordEnabled === true,
    diskFreeBytes: count(health.diskFreeBytes),
    diskTotalBytes: count(health.diskTotalBytes),
    kinds: parseKinds(health.kinds),
    uptimeSeconds: count(health.uptimeSeconds),
    planSchema: text(health.planSchema, MAX_HEALTH_STRING_CHARS),
  };
}

export function parseKinds(input: unknown): CloudKind[] {
  return Array.isArray(input) ? [...new Set(input.filter(isCloudKind))] : [];
}

function parseBlock(input: unknown): WorkerBlock | null | undefined {
  if (input === null || input === undefined) return null;
  if (!isRecord(input) || !isWorkerBlockCode(input.code)) return undefined;
  return { code: input.code, detail: text(input.detail, MAX_MESSAGE_CHARS) };
}

function parseHeartbeatJob(input: unknown): HeartbeatJob | null {
  if (!isRecord(input)) return null;
  const { id, phase } = input;
  if (typeof id !== "string" || id.length === 0 || id.length > MAX_ID_CHARS) return null;
  if (phase !== "running" && phase !== "uploading") return null;
  return {
    id,
    phase,
    attempt: positiveInt(input.attempt),
    stage: phase === "running" && isStage(input.stage) ? input.stage : null,
    percent: percent(input.percent),
    detail: optionalText(input.detail, MAX_PROGRESS_DETAIL_CHARS),
    localJobId: optionalText(input.localJobId, MAX_ID_CHARS),
  };
}

export function parseHeartbeat(input: unknown): HeartbeatBody | null {
  if (!isRecord(input) || !isWorkerState(input.state)) return null;
  const blocked = parseBlock(input.blocked);
  if (blocked === undefined) return null;
  const rawJobs = input.jobs ?? [];
  if (!Array.isArray(rawJobs)) return null;
  const jobs: HeartbeatJob[] = [];
  for (const raw of rawJobs.slice(0, MAX_HEARTBEAT_JOBS)) {
    const job = parseHeartbeatJob(raw);
    if (!job) return null;
    jobs.push(job);
  }
  return { state: input.state, blocked, health: parseWorkerHealth(input.health), jobs };
}

export interface PhaseBody {
  machineSeconds: number;
  localJobId: string | null;
}

export function parsePhase(input: unknown): PhaseBody | null {
  if (!isRecord(input) || input.phase !== "uploading") return null;
  return {
    machineSeconds: count(input.machineSeconds),
    localJobId: optionalText(input.localJobId, MAX_ID_CHARS),
  };
}

export interface ArtifactInitBody {
  name: string;
  kind: ArtifactKind;
  variant: string;
  sizeBytes: number;
  sha256: string;
}

export function parseArtifactInit(input: unknown): ArtifactInitBody | null {
  if (!isRecord(input)) return null;
  const { name, kind, variant, sizeBytes, sha256 } = input;
  if (!isArtifactKind(kind)) return null;
  if (typeof name !== "string" || !isArtifactName(name, kind)) return null;
  if (typeof variant !== "string" || !ARTIFACT_VARIANT_PATTERN.test(variant)) return null;
  if (typeof sizeBytes !== "number" || !Number.isSafeInteger(sizeBytes) || sizeBytes < 1) {
    return null;
  }
  if (!isSha256Hex(sha256)) return null;
  return { name, kind, variant, sizeBytes, sha256 };
}

export interface FailBody {
  code: FailureCode;
  // Safe to show the user.
  message: string;
  // The raw cause, for the operator only.
  detail: string;
  machineSeconds: number;
}

export function parseFail(input: unknown): FailBody | null {
  if (!isRecord(input) || !isFailureCode(input.code)) return null;
  return {
    code: input.code,
    message: text(input.message, MAX_MESSAGE_CHARS),
    detail: text(input.detail, MAX_DETAIL_CHARS),
    machineSeconds: count(input.machineSeconds),
  };
}
