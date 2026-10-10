import { NOT_CONFIGURED_CODE, SERVICE_UNAVAILABLE_CODE } from '../api/types.ts';

/** Type guards for the loopback `/api/cloud/*` documents. Unknown values degrade, never throw. */

export const CLOUD_ACCESS = ['pending', 'allowed', 'blocked'] as const;
export type CloudAccess = (typeof CLOUD_ACCESS)[number];

export const CLOUD_QUEUE_STATES = ['online', 'paused', 'offline'] as const;
export type CloudQueueState = (typeof CLOUD_QUEUE_STATES)[number];

export const CLOUD_LINK_STATUSES = ['pending', 'denied', 'expired'] as const;
export type CloudLinkStatus = (typeof CLOUD_LINK_STATUSES)[number];

export const CLOUD_ACCOUNT_ERRORS = ['portal_unreachable', 'unauthorized'] as const;
export type CloudAccountError = (typeof CLOUD_ACCOUNT_ERRORS)[number];

export const CLOUD_JOB_STATUSES = [
  'uploading_demo',
  'queued',
  'running',
  'uploading',
  'downloading',
  'done',
  'failed',
  'canceled',
] as const;
/** `unknown` is a status this Studio does not know: shown as such, never guessed. */
export type CloudJobStatus = (typeof CLOUD_JOB_STATUSES)[number] | 'unknown';

export const CLOUD_STAGES = ['downloading', 'parsing', 'capturing', 'rendering'] as const;
export type CloudStage = (typeof CLOUD_STAGES)[number];

/** Why an unfinished job is not advancing: the cloud cannot be reached, or a local transfer waits to retry. */
export const CLOUD_STALLS = ['portal_unreachable', 'demo_upload', 'download'] as const;
export type CloudStall = (typeof CLOUD_STALLS)[number];

export type CloudLink = {
  status: CloudLinkStatus;
  userCode: string;
  verifyUrl: string;
  /** Epoch ms; null when the orchestrator sent no parseable time. */
  expiresAt: number | null;
};

export type CloudQueueSummary = {
  state: CloudQueueState;
  queued: number;
  /** Seconds until a new job of each kind would start; null means no estimate. */
  waitSeconds: Record<string, number | null>;
};

export type CloudAccount = {
  portalUrl: string;
  linked: boolean;
  link: CloudLink | null;
  user: { name: string; email: string } | null;
  /** null when linked but the access value is missing or unknown. */
  access: CloudAccess | null;
  limits: { maxActive: number; dailySeconds: number } | null;
  usage: { active: number; secondsLast24h: number; secondsCommitted: number } | null;
  kinds: string[];
  queue: CloudQueueSummary | null;
  error: CloudAccountError | null;
};

export type CloudJobQueue = {
  position: number | null;
  estimatedStartAt: number | null;
  estimatedDoneAt: number | null;
  state: CloudQueueState;
};

export type CloudVideo = { name: string; variant: string; sizeBytes: number; ready: boolean };

export type CloudJob = {
  id: string;
  /** The local match (orchestrator job) this cloud job was created from. */
  localJobId: string;
  kind: string;
  title: string;
  status: CloudJobStatus;
  stage: CloudStage | null;
  percent: number | null;
  queue: CloudJobQueue | null;
  failure: { code: string; message: string } | null;
  cancelRequested: boolean;
  /** null while the job advances; with `portal_unreachable` the status is only the last one known. */
  stalled: CloudStall | null;
  createdAt: number;
  finishedAt: number | null;
  videos: CloudVideo[];
};

/** What the UI knows about the account: the document, or why there is none. */
export type CloudAccountState =
  | { kind: 'loading' }
  | { kind: 'ready'; account: CloudAccount }
  /** This Studio has no cloud client (older orchestrator, or `ZV_CLOUD_URL=off`). */
  | { kind: 'unavailable' }
  /** The local orchestrator did not answer. */
  | { kind: 'offline' };

const VIDEO_NAME_RE = /^[A-Za-z0-9][A-Za-z0-9._-]{0,119}\.mp4$/;

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function oneOf<T extends string>(values: readonly T[], value: unknown): T | null {
  return values.find((candidate) => candidate === value) ?? null;
}

function text(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function count(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
}

function percent(value: unknown): number | null {
  if (typeof value !== 'number' || !Number.isFinite(value)) return null;
  return Math.min(100, Math.max(0, Math.round(value)));
}

function time(value: unknown): number | null {
  if (typeof value !== 'string' || value === '') return null;
  const ms = Date.parse(value);
  return Number.isNaN(ms) ? null : ms;
}

function parseLink(value: unknown): CloudLink | null {
  if (!isRecord(value)) return null;
  const status = oneOf(CLOUD_LINK_STATUSES, value.status);
  if (status === null) return null;
  return {
    status,
    userCode: text(value.user_code),
    verifyUrl: text(value.verify_url),
    expiresAt: time(value.expires_at),
  };
}

function parseQueueSummary(value: unknown): CloudQueueSummary | null {
  if (!isRecord(value)) return null;
  const waitSeconds: Record<string, number | null> = {};
  if (isRecord(value.wait_seconds)) {
    for (const [kind, seconds] of Object.entries(value.wait_seconds)) {
      waitSeconds[kind] = typeof seconds === 'number' && Number.isFinite(seconds) && seconds >= 0 ? seconds : null;
    }
  }
  // An unknown state must not produce an invented estimate: treat it as offline.
  return { state: oneOf(CLOUD_QUEUE_STATES, value.state) ?? 'offline', queued: count(value.queued), waitSeconds };
}

/** Parses `GET /api/cloud/account`; null when the body is not an account document. */
export function parseCloudAccount(body: unknown): CloudAccount | null {
  if (!isRecord(body) || typeof body.linked !== 'boolean') return null;
  const linked = body.linked;
  const user = isRecord(body.user) ? { name: text(body.user.name), email: text(body.user.email) } : null;
  const limits = isRecord(body.limits)
    ? { maxActive: count(body.limits.max_active), dailySeconds: count(body.limits.daily_seconds) }
    : null;
  const usage = isRecord(body.usage)
    ? {
        active: count(body.usage.active),
        secondsLast24h: count(body.usage.seconds_last_24h),
        secondsCommitted: count(body.usage.seconds_committed),
      }
    : null;
  let error: CloudAccountError | null = null;
  if (body.error !== null && body.error !== undefined) {
    // Any error this Studio cannot name still means "the cloud did not answer".
    error = oneOf(CLOUD_ACCOUNT_ERRORS, body.error) ?? 'portal_unreachable';
  }
  return {
    portalUrl: text(body.portal_url),
    linked,
    link: linked ? null : parseLink(body.link),
    user: linked ? user : null,
    access: linked ? oneOf(CLOUD_ACCESS, body.access) : null,
    limits: linked ? limits : null,
    usage: linked ? usage : null,
    kinds: Array.isArray(body.kinds) ? body.kinds.filter((kind): kind is string => typeof kind === 'string') : [],
    queue: linked ? parseQueueSummary(body.queue) : null,
    error,
  };
}

/** Maps the account response to a state; `ok` is false for any non-2xx answer. */
export function cloudAccountState(response: { status: number; ok: boolean; body: unknown }): CloudAccountState {
  if (response.ok) {
    const account = parseCloudAccount(response.body);
    return account === null ? { kind: 'unavailable' } : { kind: 'ready', account };
  }
  const code = isRecord(response.body) ? text(response.body.code) : '';
  if (response.status === 503 && code === SERVICE_UNAVAILABLE_CODE) return { kind: 'offline' };
  if (response.status === 404 || code === NOT_CONFIGURED_CODE) return { kind: 'unavailable' };
  return { kind: 'offline' };
}

function parseJobQueue(value: unknown): CloudJobQueue | null {
  if (!isRecord(value)) return null;
  const state = oneOf(CLOUD_QUEUE_STATES, value.state) ?? 'offline';
  const position = typeof value.position === 'number' && value.position >= 1 ? Math.floor(value.position) : null;
  const online = state === 'online';
  return {
    position,
    estimatedStartAt: online ? time(value.estimated_start_at) : null,
    estimatedDoneAt: online ? time(value.estimated_done_at) : null,
    state,
  };
}

function parseVideos(value: unknown): CloudVideo[] {
  if (!Array.isArray(value)) return [];
  const videos: CloudVideo[] = [];
  for (const item of value) {
    // A name that is not a plain file name can never become a URL segment.
    if (!isRecord(item) || typeof item.name !== 'string' || !VIDEO_NAME_RE.test(item.name)) continue;
    videos.push({
      name: item.name,
      variant: text(item.variant),
      sizeBytes: count(item.size_bytes),
      ready: item.ready === true,
    });
  }
  return videos;
}

/** Parses one `LocalCloudJob`; null when it has no usable identity. */
export function parseCloudJob(body: unknown): CloudJob | null {
  if (!isRecord(body) || typeof body.id !== 'string' || body.id === '') return null;
  const status: CloudJobStatus = oneOf(CLOUD_JOB_STATUSES, body.status) ?? 'unknown';
  const failure = isRecord(body.failure)
    ? { code: text(body.failure.code), message: text(body.failure.message) }
    : null;
  return {
    id: body.id,
    localJobId: text(body.local_job_id),
    kind: text(body.kind) || 'short',
    title: text(body.title).trim() || 'Short en la nube',
    status,
    stage: status === 'running' ? oneOf(CLOUD_STAGES, body.stage) : null,
    percent: percent(body.percent),
    queue: status === 'queued' ? parseJobQueue(body.queue) : null,
    // A failed job with no failure object still needs a line to show.
    failure: status === 'failed' ? (failure ?? { code: '', message: '' }) : null,
    cancelRequested: body.cancel_requested === true,
    stalled: oneOf(CLOUD_STALLS, body.stalled),
    createdAt: time(body.created_at) ?? 0,
    finishedAt: time(body.finished_at),
    videos: parseVideos(body.videos),
  };
}

/** Parses `GET /api/cloud/jobs`, dropping entries that are not jobs. */
export function parseCloudJobs(body: unknown): CloudJob[] {
  if (!isRecord(body) || !Array.isArray(body.jobs)) return [];
  return body.jobs.flatMap((item) => {
    const job = parseCloudJob(item);
    return job === null ? [] : [job];
  });
}
