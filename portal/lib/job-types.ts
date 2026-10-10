export const JOB_KINDS = ["manual", "short", "full_demo"] as const;
export type JobKind = (typeof JOB_KINDS)[number];

export const CLOUD_KINDS = ["short", "full_demo"] as const;
export type CloudKind = (typeof CLOUD_KINDS)[number];

export const JOB_STATUSES = [
  "awaiting_demo",
  "queued",
  "running",
  "uploading",
  "done",
  "failed",
  "canceled",
] as const;
export type JobStatus = (typeof JOB_STATUSES)[number];

// Statuses of kind "manual" rows created before the queue existed.
export const LEGACY_STATUSES = [
  "pending",
  "approved",
  "processing",
  "rejected",
] as const;
export type LegacyStatus = (typeof LEGACY_STATUSES)[number];

export const ACTIVE_STATUSES = [
  "awaiting_demo",
  "queued",
  "running",
  "uploading",
] as const;
export type ActiveStatus = (typeof ACTIVE_STATUSES)[number];

export const TERMINAL_STATUSES = [
  "done",
  "failed",
  "canceled",
  "rejected",
] as const;
export type TerminalStatus = (typeof TERMINAL_STATUSES)[number];

export const LEASED_STATUSES = ["running", "uploading"] as const;
export type LeasedStatus = (typeof LEASED_STATUSES)[number];

export const STAGES = [
  "downloading",
  "parsing",
  "capturing",
  "rendering",
] as const;
export type Stage = (typeof STAGES)[number];

export const RETRY_CODES = [
  "capture_flake",
  "interrupted",
  "worker_lost",
  "demo_download_failed",
  "internal",
] as const;
export const TERMINAL_CODES = [
  "demo_incompatible",
  "unplayable_start",
  "target_not_found",
  "spec_mismatch",
  "invalid_spec",
  "render_failed",
  "job_failed",
  "timeout",
  "upload_failed",
] as const;
export const MACHINE_CODES = [
  "capture_incompatible",
  "steam_unavailable",
  "disk_full",
  "tools_missing",
] as const;
export const CANCEL_CODES = ["canceled"] as const;

export const FAILURE_CODES = [
  ...RETRY_CODES,
  ...TERMINAL_CODES,
  ...MACHINE_CODES,
  ...CANCEL_CODES,
] as const;
export type FailureCode = (typeof FAILURE_CODES)[number];

export const FAILURE_CLASSES = [
  "retry",
  "terminal",
  "machine",
  "cancel",
] as const;
export type FailureClass = (typeof FAILURE_CLASSES)[number];

export const WORKER_STATES = ["idle", "busy", "blocked"] as const;
export type WorkerState = (typeof WORKER_STATES)[number];

export const WORKER_BLOCK_CODES = [
  "steam_unavailable",
  "disk_full",
  "tools_missing",
  "cs2_running",
] as const;
export type WorkerBlockCode = (typeof WORKER_BLOCK_CODES)[number];

export const PAUSE_SOURCES = ["admin", "auto"] as const;
export type PauseSource = (typeof PAUSE_SOURCES)[number];

export const CLOUD_ACCESS = ["pending", "allowed", "blocked"] as const;
export type CloudAccess = (typeof CLOUD_ACCESS)[number];

export const QUEUE_STATES = ["online", "paused", "offline"] as const;
export type QueueState = (typeof QUEUE_STATES)[number];

export const ARTIFACT_KINDS = ["video", "cover"] as const;
export type ArtifactKind = (typeof ARTIFACT_KINDS)[number];

export const ARTIFACT_STATUSES = ["uploading", "ready"] as const;
export type ArtifactStatus = (typeof ARTIFACT_STATUSES)[number];

export const LINK_STATUSES = [
  "pending",
  "approved",
  "consumed",
  "denied",
] as const;
export type LinkStatus = (typeof LINK_STATUSES)[number];

export const CANCEL_ACTORS = ["user", "admin"] as const;
export type CancelActor = (typeof CANCEL_ACTORS)[number];

export const EVENT_TYPES = [
  "created",
  "demo_uploaded",
  "queued",
  "claimed",
  "stage",
  "phase_uploading",
  "done",
  "failed",
  "requeued",
  "lease_expired",
  "cancel_requested",
  "canceled",
  "retried",
  "boosted",
  "worker_created",
  "worker_paused",
  "worker_resumed",
  "worker_auto_paused",
  "worker_revoked",
  "user_access_changed",
  "user_limits_changed",
  "device_linked",
  "device_revoked",
] as const;
export type EventType = (typeof EVENT_TYPES)[number];

function oneOf<T extends string>(
  list: readonly T[],
): (value: unknown) => value is T {
  return (value: unknown): value is T =>
    typeof value === "string" && list.some((item) => item === value);
}

export const isJobKind = oneOf(JOB_KINDS);
export const isCloudKind = oneOf(CLOUD_KINDS);
export const isJobStatus = oneOf(JOB_STATUSES);
export const isLegacyStatus = oneOf(LEGACY_STATUSES);
export const isActiveStatus = oneOf(ACTIVE_STATUSES);
export const isTerminalStatus = oneOf(TERMINAL_STATUSES);
export const isLeasedStatus = oneOf(LEASED_STATUSES);
export const isStage = oneOf(STAGES);
export const isFailureCode = oneOf(FAILURE_CODES);
export const isRetryCode = oneOf(RETRY_CODES);
export const isTerminalCode = oneOf(TERMINAL_CODES);
export const isMachineCode = oneOf(MACHINE_CODES);
export const isWorkerState = oneOf(WORKER_STATES);
export const isWorkerBlockCode = oneOf(WORKER_BLOCK_CODES);
export const isPauseSource = oneOf(PAUSE_SOURCES);
export const isCloudAccess = oneOf(CLOUD_ACCESS);
export const isArtifactKind = oneOf(ARTIFACT_KINDS);
export const isArtifactStatus = oneOf(ARTIFACT_STATUSES);
export const isLinkStatus = oneOf(LINK_STATUSES);
export const isCancelActor = oneOf(CANCEL_ACTORS);

export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
