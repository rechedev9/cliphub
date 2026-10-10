// Response types of the user API (contract section H) and the admin API (section I).

export const JOB_KINDS = ["manual", "short", "full_demo"] as const;
export type JobKind = (typeof JOB_KINDS)[number];

export const JOB_STATUSES = [
  "awaiting_demo",
  "queued",
  "running",
  "uploading",
  "done",
  "failed",
  "canceled",
  "pending",
  "approved",
  "processing",
  "rejected",
] as const;
// Includes the legacy statuses, which only rows of kind "manual" carry.
export type JobStatus = (typeof JOB_STATUSES)[number];

export const STAGES = ["downloading", "parsing", "capturing", "rendering"] as const;
export type Stage = (typeof STAGES)[number];

export const FAILURE_CODES = [
  "capture_flake",
  "interrupted",
  "worker_lost",
  "demo_download_failed",
  "internal",
  "demo_incompatible",
  "unplayable_start",
  "target_not_found",
  "spec_mismatch",
  "invalid_spec",
  "render_failed",
  "job_failed",
  "timeout",
  "upload_failed",
  "capture_incompatible",
  "steam_unavailable",
  "disk_full",
  "tools_missing",
  "canceled",
] as const;
export type FailureCode = (typeof FAILURE_CODES)[number];

export const WORKER_STATES = ["idle", "busy", "blocked"] as const;
export type WorkerState = (typeof WORKER_STATES)[number];

export const PAUSE_SOURCES = ["admin", "auto"] as const;
export type PauseSource = (typeof PAUSE_SOURCES)[number];

export const CLOUD_ACCESS = ["pending", "allowed", "blocked"] as const;
export type CloudAccess = (typeof CLOUD_ACCESS)[number];

export const QUEUE_STATES = ["online", "paused", "offline"] as const;
export type QueueState = (typeof QUEUE_STATES)[number];

export const ARTIFACT_KINDS = ["video", "cover"] as const;
export type ArtifactKind = (typeof ARTIFACT_KINDS)[number];

export const CANCELED_BY = ["user", "admin"] as const;
export type CanceledBy = (typeof CANCELED_BY)[number];

export interface JobQueueInfo {
  position: number;
  estimatedStartAt: number | null;
  estimatedDoneAt: number | null;
  state: QueueState;
}

export interface JobFailure {
  code: FailureCode;
  message: string;
}

export interface JobArtifact {
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
  kind: JobKind;
  title: string | null;
  status: JobStatus;
  stage: Stage | null;
  progressPercent: number | null;
  createdAt: number;
  enqueuedAt: number | null;
  startedAt: number | null;
  finishedAt: number | null;
  estimatedSeconds: number | null;
  attempt: number;
  cancelRequested: boolean;
  queue: JobQueueInfo | null;
  failure: JobFailure | null;
  artifacts: JobArtifact[];
}

export interface StudioJobsResponse {
  jobs: StudioJob[];
}

export interface StudioMe {
  user: { id: string; name: string | null; email: string | null; image: string | null };
  access: CloudAccess;
  limits: { maxActive: number; dailySeconds: number; maxDemoBytes: number };
  usage: { active: number; secondsLast24h: number; secondsCommitted: number };
  kinds: string[];
  queue: { state: QueueState; queued: number };
}

export interface StudioDevice {
  id: string;
  name: string;
  createdAt: number;
  lastSeenAt: number | null;
}

export interface StudioDevicesResponse {
  devices: StudioDevice[];
}

export interface LinkInfo {
  deviceName: string;
  expiresAt: number;
  // False when the code was asked for from a different address than this browser's.
  sameNetwork: boolean;
}

export interface WorkerHealth {
  hostname: string;
  studioVersion: string;
  cs2PatchVersion: string;
  hlaeVersion: string;
  steamRunning: boolean;
  recordEnabled: boolean;
  diskFreeBytes: number;
  diskTotalBytes: number;
  kinds: string[];
  uptimeSeconds: number;
  // Kill plan schema of the worker's Studio; empty when it did not report one.
  planSchema: string;
}

export interface AdminWorker {
  id: string;
  name: string;
  online: boolean;
  lastSeenAt: number | null;
  state: WorkerState | null;
  blocked: { code: string; detail: string } | null;
  paused: boolean;
  pausedBy: PauseSource | null;
  pauseReason: string | null;
  revoked: boolean;
  createdAt: number;
  health: WorkerHealth | null;
  currentJobId: string | null;
}

export interface AdminJob extends StudioJob {
  user: { id: string; name: string | null; email: string | null };
  progressDetail: string | null;
  workerId: string | null;
  localJobId: string | null;
  leaseExpiresAt: number | null;
  priorityBoost: number;
  machineSeconds: number;
  maxAttempts: number;
  waitedSeconds: number | null;
  failureDetail: string | null;
  canceledBy: CanceledBy | null;
  demo: { fileName: string | null; sizeBytes: number | null; sha256: string | null; present: boolean };
  legacyStatus: boolean;
}

export interface AdminStats {
  queued: number;
  done24h: number;
  failed24h: number;
  canceled24h: number;
  medianWaitSeconds24h: number | null;
  machineSeconds24h: number;
  storage: { freeBytes: number; totalBytes: number; demoBytes: number; artifactBytes: number };
}

export interface AdminOverview {
  now: number;
  workers: AdminWorker[];
  running: AdminJob[];
  uploading: AdminJob[];
  queue: AdminJob[];
  stats: AdminStats;
  pendingUsers: number;
}

export interface AdminJobsResponse {
  jobs: AdminJob[];
  nextBefore: number | null;
}

export interface AdminEvent {
  id: string;
  at: number;
  actor: string;
  type: string;
  detail: string | null;
}

export interface AdminActivityEvent extends AdminEvent {
  requestId: string | null;
  workerId: string | null;
  subjectUserId: string | null;
}

export interface AdminEventsResponse {
  events: AdminActivityEvent[];
}

// The parts of JobSpec (contract section D) the panel summarises.
export interface JobSpecSummary {
  targetSteamId: string;
  tickrate: number;
  windows: { id: string; tickStart: number; tickEnd: number }[];
  preset: string | null;
  // Kill plan schema of the Studio that sent the job; null when it did not say.
  planSchema: string | null;
}

export interface AdminJobDetail {
  job: AdminJob;
  spec: JobSpecSummary | null;
  events: AdminEvent[];
}

export interface AdminWorkersResponse {
  workers: AdminWorker[];
}

export interface AdminWorkerCreated {
  worker: AdminWorker;
  token: string;
}

export interface AdminUser {
  id: string;
  name: string | null;
  email: string | null;
  image: string | null;
  access: CloudAccess;
  isAdmin: boolean;
  limits: { maxActive: number; dailySeconds: number };
  overrides: { maxActive: number | null; dailySeconds: number | null };
  usage: { active: number; secondsLast24h: number; jobs7d: number; failed7d: number };
  devices: number;
  lastJobAt: number | null;
  note: string | null;
}

export interface AdminUsersResponse {
  users: AdminUser[];
}
