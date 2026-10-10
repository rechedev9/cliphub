import {
  ARTIFACT_KINDS,
  CANCELED_BY,
  CLOUD_ACCESS,
  FAILURE_CODES,
  JOB_KINDS,
  JOB_STATUSES,
  PAUSE_SOURCES,
  QUEUE_STATES,
  STAGES,
  WORKER_STATES,
} from "./api-types.ts";
import type {
  AdminActivityEvent,
  AdminEvent,
  AdminEventsResponse,
  AdminJob,
  AdminJobDetail,
  AdminJobsResponse,
  AdminOverview,
  AdminUser,
  AdminUsersResponse,
  AdminWorker,
  AdminWorkerCreated,
  AdminWorkersResponse,
  JobArtifact,
  JobSpecSummary,
  LinkInfo,
  StudioDevice,
  StudioDevicesResponse,
  StudioJob,
  StudioJobsResponse,
  StudioMe,
  WorkerHealth,
} from "./api-types.ts";

// A decoder returns the validated value or throws a ParseError naming the bad field.
type Decoder<T> = (value: unknown, path: string) => T;

export class ParseError extends Error {
  constructor(path: string, expected: string) {
    super(`${path}: se esperaba ${expected}`);
    this.name = "ParseError";
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

const raw: Decoder<unknown> = (value) => value;

const str: Decoder<string> = (value, path) => {
  if (typeof value !== "string") throw new ParseError(path, "texto");
  return value;
};

const num: Decoder<number> = (value, path) => {
  if (typeof value !== "number" || !Number.isFinite(value)) throw new ParseError(path, "número");
  return value;
};

const bool: Decoder<boolean> = (value, path) => {
  if (typeof value !== "boolean") throw new ParseError(path, "booleano");
  return value;
};

function nullable<T>(decode: Decoder<T>): Decoder<T | null> {
  return (value, path) => (value === null || value === undefined ? null : decode(value, path));
}

// The portal sends "" and 0 where a legacy row has no value; the views expect null there.
const optionalText: Decoder<string | null> = (value, path) => {
  const text = nullable(str)(value, path);
  return text === null || text.trim() === "" ? null : text;
};

const optionalSize: Decoder<number | null> = (value, path) => {
  const size = nullable(num)(value, path);
  return size === null || size <= 0 ? null : size;
};

function list<T>(decode: Decoder<T>): Decoder<T[]> {
  return (value, path) => {
    if (!Array.isArray(value)) throw new ParseError(path, "lista");
    return value.map((item, index) => decode(item, `${path}[${index}]`));
  };
}

function oneOf<T extends string>(allowed: readonly T[]): Decoder<T> {
  return (value, path) => {
    const match = allowed.find((candidate) => candidate === value);
    if (match === undefined) throw new ParseError(path, `uno de ${allowed.join(", ")}`);
    return match;
  };
}

interface Reader {
  get<T>(key: string, decode: Decoder<T>): T;
  child(key: string): Reader;
}

function reader(value: unknown, path: string): Reader {
  if (!isRecord(value)) throw new ParseError(path, "objeto");
  return {
    get: (key, decode) => decode(value[key], `${path}.${key}`),
    child: (key) => reader(value[key], `${path}.${key}`),
  };
}

const jobArtifact: Decoder<JobArtifact> = (value, path) => {
  const r = reader(value, path);
  return {
    id: r.get("id", str),
    kind: r.get("kind", oneOf(ARTIFACT_KINDS)),
    variant: r.get("variant", str),
    name: r.get("name", str),
    sizeBytes: r.get("sizeBytes", num),
    sha256: r.get("sha256", str),
    expiresAt: r.get("expiresAt", num),
  };
};

const studioJob: Decoder<StudioJob> = (value, path) => {
  const r = reader(value, path);
  return {
    id: r.get("id", str),
    kind: r.get("kind", oneOf(JOB_KINDS)),
    title: r.get("title", optionalText),
    status: r.get("status", oneOf(JOB_STATUSES)),
    stage: r.get("stage", nullable(oneOf(STAGES))),
    progressPercent: r.get("progressPercent", nullable(num)),
    createdAt: r.get("createdAt", num),
    enqueuedAt: r.get("enqueuedAt", nullable(num)),
    startedAt: r.get("startedAt", nullable(num)),
    finishedAt: r.get("finishedAt", nullable(num)),
    estimatedSeconds: r.get("estimatedSeconds", nullable(num)),
    attempt: r.get("attempt", num),
    cancelRequested: r.get("cancelRequested", bool),
    queue: r.get(
      "queue",
      nullable((queueValue, queuePath) => {
        const q = reader(queueValue, queuePath);
        return {
          position: q.get("position", num),
          estimatedStartAt: q.get("estimatedStartAt", nullable(num)),
          estimatedDoneAt: q.get("estimatedDoneAt", nullable(num)),
          state: q.get("state", oneOf(QUEUE_STATES)),
        };
      }),
    ),
    failure: r.get(
      "failure",
      nullable((failureValue, failurePath) => {
        const f = reader(failureValue, failurePath);
        return { code: f.get("code", oneOf(FAILURE_CODES)), message: f.get("message", str) };
      }),
    ),
    artifacts: r.get("artifacts", list(jobArtifact)),
  };
};

const workerHealth: Decoder<WorkerHealth> = (value, path) => {
  const r = reader(value, path);
  return {
    hostname: r.get("hostname", str),
    studioVersion: r.get("studioVersion", str),
    cs2PatchVersion: r.get("cs2PatchVersion", str),
    hlaeVersion: r.get("hlaeVersion", str),
    steamRunning: r.get("steamRunning", bool),
    recordEnabled: r.get("recordEnabled", bool),
    diskFreeBytes: r.get("diskFreeBytes", num),
    diskTotalBytes: r.get("diskTotalBytes", num),
    kinds: r.get("kinds", list(str)),
    uptimeSeconds: r.get("uptimeSeconds", num),
    planSchema: r.get("planSchema", str),
  };
};

const adminWorker: Decoder<AdminWorker> = (value, path) => {
  const r = reader(value, path);
  return {
    id: r.get("id", str),
    name: r.get("name", str),
    online: r.get("online", bool),
    lastSeenAt: r.get("lastSeenAt", nullable(num)),
    state: r.get("state", nullable(oneOf(WORKER_STATES))),
    blocked: r.get(
      "blocked",
      nullable((blockedValue, blockedPath) => {
        const b = reader(blockedValue, blockedPath);
        return { code: b.get("code", str), detail: b.get("detail", str) };
      }),
    ),
    paused: r.get("paused", bool),
    pausedBy: r.get("pausedBy", nullable(oneOf(PAUSE_SOURCES))),
    pauseReason: r.get("pauseReason", nullable(str)),
    revoked: r.get("revoked", bool),
    createdAt: r.get("createdAt", num),
    health: r.get("health", nullable(workerHealth)),
    currentJobId: r.get("currentJobId", nullable(str)),
  };
};

const adminJob: Decoder<AdminJob> = (value, path) => {
  const r = reader(value, path);
  const user = r.child("user");
  const demo = r.child("demo");
  return {
    ...studioJob(value, path),
    user: {
      id: user.get("id", str),
      name: user.get("name", nullable(str)),
      email: user.get("email", nullable(str)),
    },
    progressDetail: r.get("progressDetail", optionalText),
    workerId: r.get("workerId", nullable(str)),
    localJobId: r.get("localJobId", nullable(str)),
    leaseExpiresAt: r.get("leaseExpiresAt", nullable(num)),
    priorityBoost: r.get("priorityBoost", num),
    machineSeconds: r.get("machineSeconds", num),
    maxAttempts: r.get("maxAttempts", num),
    waitedSeconds: r.get("waitedSeconds", nullable(num)),
    failureDetail: r.get("failureDetail", nullable(str)),
    canceledBy: r.get("canceledBy", nullable(oneOf(CANCELED_BY))),
    demo: {
      fileName: demo.get("fileName", optionalText),
      sizeBytes: demo.get("sizeBytes", optionalSize),
      sha256: demo.get("sha256", optionalText),
      present: demo.get("present", bool),
    },
    legacyStatus: r.get("legacyStatus", bool),
  };
};

const adminEvent: Decoder<AdminEvent> = (value, path) => {
  const r = reader(value, path);
  return {
    id: r.get("id", str),
    at: r.get("at", num),
    actor: r.get("actor", str),
    type: r.get("type", str),
    detail: r.get("detail", nullable(str)),
  };
};

const activityEvent: Decoder<AdminActivityEvent> = (value, path) => {
  const r = reader(value, path);
  return {
    ...adminEvent(value, path),
    requestId: r.get("requestId", nullable(str)),
    workerId: r.get("workerId", nullable(str)),
    subjectUserId: r.get("subjectUserId", nullable(str)),
  };
};

// Only the fields the panel shows are read; the rest of the spec is opaque here.
const jobSpec: Decoder<JobSpecSummary> = (value, path) => {
  const r = reader(value, path);
  const capture = r.child("capture");
  const generate = r.get("generate", raw);
  const client = r.get("client", raw);
  const planSchema = isRecord(client) ? client.planSchema : null;
  return {
    targetSteamId: r.get("targetSteamId", str),
    tickrate: capture.get("tickrate", num),
    windows: capture.get(
      "windows",
      list((windowValue, windowPath) => {
        const w = reader(windowValue, windowPath);
        return {
          id: w.get("id", str),
          tickStart: w.get("tickStart", num),
          tickEnd: w.get("tickEnd", num),
        };
      }),
    ),
    preset: isRecord(generate) && typeof generate.preset === "string" ? generate.preset : null,
    planSchema: typeof planSchema === "string" && planSchema !== "" ? planSchema : null,
  };
};

const adminUser: Decoder<AdminUser> = (value, path) => {
  const r = reader(value, path);
  const limits = r.child("limits");
  const overrides = r.child("overrides");
  const usage = r.child("usage");
  return {
    id: r.get("id", str),
    name: r.get("name", nullable(str)),
    email: r.get("email", nullable(str)),
    image: r.get("image", nullable(str)),
    access: r.get("access", oneOf(CLOUD_ACCESS)),
    isAdmin: r.get("isAdmin", bool),
    limits: { maxActive: limits.get("maxActive", num), dailySeconds: limits.get("dailySeconds", num) },
    overrides: {
      maxActive: overrides.get("maxActive", nullable(num)),
      dailySeconds: overrides.get("dailySeconds", nullable(num)),
    },
    usage: {
      active: usage.get("active", num),
      secondsLast24h: usage.get("secondsLast24h", num),
      jobs7d: usage.get("jobs7d", num),
      failed7d: usage.get("failed7d", num),
    },
    devices: r.get("devices", num),
    lastJobAt: r.get("lastJobAt", nullable(num)),
    note: r.get("note", nullable(str)),
  };
};

const studioDevice: Decoder<StudioDevice> = (value, path) => {
  const r = reader(value, path);
  return {
    id: r.get("id", str),
    name: r.get("name", str),
    createdAt: r.get("createdAt", num),
    lastSeenAt: r.get("lastSeenAt", nullable(num)),
  };
};

export function parseOverview(value: unknown): AdminOverview {
  const r = reader(value, "overview");
  const stats = r.child("stats");
  const storage = stats.child("storage");
  return {
    now: r.get("now", num),
    workers: r.get("workers", list(adminWorker)),
    running: r.get("running", list(adminJob)),
    uploading: r.get("uploading", list(adminJob)),
    queue: r.get("queue", list(adminJob)),
    stats: {
      queued: stats.get("queued", num),
      done24h: stats.get("done24h", num),
      failed24h: stats.get("failed24h", num),
      canceled24h: stats.get("canceled24h", num),
      medianWaitSeconds24h: stats.get("medianWaitSeconds24h", nullable(num)),
      machineSeconds24h: stats.get("machineSeconds24h", num),
      storage: {
        freeBytes: storage.get("freeBytes", num),
        totalBytes: storage.get("totalBytes", num),
        demoBytes: storage.get("demoBytes", num),
        artifactBytes: storage.get("artifactBytes", num),
      },
    },
    pendingUsers: r.get("pendingUsers", num),
  };
}

export function parseAdminJobs(value: unknown): AdminJobsResponse {
  const r = reader(value, "jobs");
  return { jobs: r.get("jobs", list(adminJob)), nextBefore: r.get("nextBefore", nullable(num)) };
}

export function parseAdminJobDetail(value: unknown): AdminJobDetail {
  const r = reader(value, "detail");
  return {
    job: r.get("job", adminJob),
    spec: r.get("spec", nullable(jobSpec)),
    events: r.get("events", list(adminEvent)),
  };
}

export function parseAdminWorkers(value: unknown): AdminWorkersResponse {
  return { workers: reader(value, "workers").get("workers", list(adminWorker)) };
}

export function parseAdminWorkerCreated(value: unknown): AdminWorkerCreated {
  const r = reader(value, "created");
  return { worker: r.get("worker", adminWorker), token: r.get("token", str) };
}

export function parseAdminUsers(value: unknown): AdminUsersResponse {
  return { users: reader(value, "users").get("users", list(adminUser)) };
}

export function parseAdminEvents(value: unknown): AdminEventsResponse {
  return { events: reader(value, "events").get("events", list(activityEvent)) };
}

export function parseStudioJobs(value: unknown): StudioJobsResponse {
  return { jobs: reader(value, "jobs").get("jobs", list(studioJob)) };
}

export function parseStudioMe(value: unknown): StudioMe {
  const r = reader(value, "me");
  const user = r.child("user");
  const limits = r.child("limits");
  const usage = r.child("usage");
  const queue = r.child("queue");
  return {
    user: {
      id: user.get("id", str),
      name: user.get("name", nullable(str)),
      email: user.get("email", nullable(str)),
      image: user.get("image", nullable(str)),
    },
    access: r.get("access", oneOf(CLOUD_ACCESS)),
    limits: {
      maxActive: limits.get("maxActive", num),
      dailySeconds: limits.get("dailySeconds", num),
      maxDemoBytes: limits.get("maxDemoBytes", num),
    },
    usage: {
      active: usage.get("active", num),
      secondsLast24h: usage.get("secondsLast24h", num),
      secondsCommitted: usage.get("secondsCommitted", num),
    },
    kinds: r.get("kinds", list(str)),
    queue: { state: queue.get("state", oneOf(QUEUE_STATES)), queued: queue.get("queued", num) },
  };
}

export function parseStudioDevices(value: unknown): StudioDevicesResponse {
  return { devices: reader(value, "devices").get("devices", list(studioDevice)) };
}

export function parseLinkInfo(value: unknown): LinkInfo {
  const r = reader(value, "link");
  return {
    deviceName: r.get("deviceName", str),
    expiresAt: r.get("expiresAt", num),
    sameNetwork: r.get("sameNetwork", bool),
  };
}

// Error bodies are `{ error, code? }`; the code is preferred because the UI maps it to copy.
export function parseErrorCode(value: unknown): string | null {
  if (!isRecord(value)) return null;
  if (typeof value.code === "string" && value.code !== "") return value.code;
  if (typeof value.error === "string" && value.error !== "") return value.error;
  return null;
}
