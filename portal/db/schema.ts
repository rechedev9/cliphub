import {
  sqliteTable,
  text,
  integer,
  index,
  primaryKey,
  uniqueIndex,
} from "drizzle-orm/sqlite-core";
import type { AdapterAccountType } from "next-auth/adapters";

// Auth.js's own required tables (schema shape mandated by @auth/drizzle-adapter).

export const users = sqliteTable("user", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  name: text("name"),
  email: text("email").unique(),
  emailVerified: integer("emailVerified", { mode: "timestamp_ms" }),
  image: text("image"),
});

export const accounts = sqliteTable(
  "account",
  {
    userId: text("userId")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    type: text("type").$type<AdapterAccountType>().notNull(),
    provider: text("provider").notNull(),
    providerAccountId: text("providerAccountId").notNull(),
    refresh_token: text("refresh_token"),
    access_token: text("access_token"),
    expires_at: integer("expires_at"),
    token_type: text("token_type"),
    scope: text("scope"),
    id_token: text("id_token"),
    session_state: text("session_state"),
  },
  (account) => [
    primaryKey({ columns: [account.provider, account.providerAccountId] }),
  ],
);

export const sessions = sqliteTable("session", {
  sessionToken: text("sessionToken").primaryKey(),
  userId: text("userId")
    .notNull()
    .references(() => users.id, { onDelete: "cascade" }),
  expires: integer("expires", { mode: "timestamp_ms" }).notNull(),
});

export const verificationTokens = sqliteTable(
  "verificationToken",
  {
    identifier: text("identifier").notNull(),
    token: text("token").notNull(),
    expires: integer("expires", { mode: "timestamp_ms" }).notNull(),
  },
  (vt) => [primaryKey({ columns: [vt.identifier, vt.token] })],
);

// The capture machines that pull jobs. The token is stored only as its sha256.
export const workers = sqliteTable("worker", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  name: text("name").notNull(),
  tokenHash: text("tokenHash").notNull().unique(),
  createdAt: integer("createdAt").notNull(),
  revokedAt: integer("revokedAt"),
  paused: integer("paused", { mode: "boolean" }).notNull().default(false),
  pausedBy: text("pausedBy"),
  pauseReason: text("pauseReason"),
  lastSeenAt: integer("lastSeenAt"),
  state: text("state"),
  blockedCode: text("blockedCode"),
  blockedDetail: text("blockedDetail"),
  health: text("health"),
  consecutiveFailures: integer("consecutiveFailures").notNull().default(0),
});

// A linked ClipHub Studio install. The token is stored only as its sha256.
export const devices = sqliteTable("device", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  userId: text("userId")
    .notNull()
    .references(() => users.id, { onDelete: "cascade" }),
  name: text("name").notNull(),
  tokenHash: text("tokenHash").notNull().unique(),
  createdAt: integer("createdAt").notNull(),
  lastSeenAt: integer("lastSeenAt"),
  revokedAt: integer("revokedAt"),
});

export const deviceLinks = sqliteTable("device_link", {
  id: text("id")
    .primaryKey()
    .$defaultFn(() => crypto.randomUUID()),
  userCode: text("userCode").notNull().unique(),
  pollTokenHash: text("pollTokenHash").notNull(),
  deviceName: text("deviceName").notNull(),
  status: text("status").notNull().default("pending"),
  userId: text("userId").references(() => users.id, { onDelete: "cascade" }),
  createdAt: integer("createdAt").notNull(),
  expiresAt: integer("expiresAt").notNull(),
  // Salted hash of the address that asked for the code; null on links from before it existed.
  startIpHash: text("startIpHash"),
});

// Cloud access and limit overrides, kept apart from the Auth.js user table.
export const userCloud = sqliteTable("user_cloud", {
  userId: text("userId")
    .primaryKey()
    .references(() => users.id, { onDelete: "cascade" }),
  access: text("access").notNull(),
  maxActive: integer("maxActive"),
  dailySeconds: integer("dailySeconds"),
  note: text("note"),
  updatedAt: integer("updatedAt").notNull(),
});

// The cloud job. Rows from before the queue existed stay as kind "manual".
// Columns declared as plain integers hold epoch milliseconds.
export const requests = sqliteTable(
  "request",
  {
    id: text("id")
      .primaryKey()
      .$defaultFn(() => crypto.randomUUID()),
    userId: text("userId")
      .notNull()
      .references(() => users.id, { onDelete: "cascade" }),
    status: text("status").notNull().default("awaiting_demo"),
    note: text("note"),
    demoPath: text("demoPath"),
    demoSha256: text("demoSha256"),
    demoOriginalName: text("demoOriginalName"),
    finalVideoPath: text("finalVideoPath"),
    finalVideoName: text("finalVideoName"),
    failureReason: text("failureReason"),
    localJobId: text("localJobId"),
    // Start of the current attempt.
    claimedAt: integer("claimedAt", { mode: "timestamp_ms" }),
    createdAt: integer("createdAt", { mode: "timestamp_ms" })
      .notNull()
      .$defaultFn(() => new Date()),
    updatedAt: integer("updatedAt", { mode: "timestamp_ms" })
      .notNull()
      .$defaultFn(() => new Date()),
    kind: text("kind").notNull().default("manual"),
    title: text("title"),
    spec: text("spec"),
    targetSteamId: text("targetSteamId"),
    demoSizeBytes: integer("demoSizeBytes"),
    deviceId: text("deviceId").references(() => devices.id, {
      onDelete: "set null",
    }),
    estCaptureSeconds: integer("estCaptureSeconds"),
    estimatedSeconds: integer("estimatedSeconds"),
    // Set once and kept across requeues, so a retry keeps its seniority.
    enqueuedAt: integer("enqueuedAt"),
    priorityBoost: integer("priorityBoost").notNull().default(0),
    workerId: text("workerId").references(() => workers.id, {
      onDelete: "set null",
    }),
    leaseExpiresAt: integer("leaseExpiresAt"),
    attempt: integer("attempt").notNull().default(0),
    maxAttempts: integer("maxAttempts").notNull().default(2),
    stage: text("stage"),
    progressPercent: integer("progressPercent"),
    progressDetail: text("progressDetail"),
    finishedAt: integer("finishedAt"),
    machineSeconds: integer("machineSeconds").notNull().default(0),
    failureCode: text("failureCode"),
    failureDetail: text("failureDetail"),
    cancelRequestedAt: integer("cancelRequestedAt"),
    canceledBy: text("canceledBy"),
    // When the current attempt moved to uploading.
    uploadingAt: integer("uploadingAt"),
    // Times a machine fault sent this job back to the queue; reset by an operator retry.
    machineRequeues: integer("machineRequeues").notNull().default(0),
  },
  (request) => [
    index("request_status_enqueued").on(request.status, request.enqueuedAt),
    index("request_user_created").on(request.userId, request.createdAt),
    index("request_worker_status").on(request.workerId, request.status),
  ],
);

// One result file of a job, uploaded by the worker in resumable parts.
export const requestArtifacts = sqliteTable(
  "request_artifact",
  {
    id: text("id")
      .primaryKey()
      .$defaultFn(() => crypto.randomUUID()),
    requestId: text("requestId")
      .notNull()
      .references(() => requests.id, { onDelete: "cascade" }),
    variant: text("variant").notNull(),
    name: text("name").notNull(),
    path: text("path").notNull(),
    sizeBytes: integer("sizeBytes").notNull(),
    uploadedAt: integer("uploadedAt", { mode: "timestamp_ms" })
      .notNull()
      .$defaultFn(() => new Date()),
    kind: text("kind").notNull().default("video"),
    sha256: text("sha256"),
    status: text("status").notNull().default("ready"),
    partSize: integer("partSize"),
    // JSON array of the 1-based part numbers already written.
    receivedParts: text("receivedParts"),
    receivedAt: integer("receivedAt"),
  },
  // Re-sending the same file must replace its row, not pile up beside it.
  (artifact) => [
    uniqueIndex("request_artifact_unique").on(
      artifact.requestId,
      artifact.variant,
      artifact.name,
    ),
  ],
);

// Audit trail and job timeline. actor is "system", "worker:<id>", "admin:<userId>" or "user:<userId>".
export const events = sqliteTable(
  "event",
  {
    id: text("id")
      .primaryKey()
      .$defaultFn(() => crypto.randomUUID()),
    at: integer("at").notNull(),
    requestId: text("requestId").references(() => requests.id, {
      onDelete: "set null",
    }),
    workerId: text("workerId").references(() => workers.id, {
      onDelete: "set null",
    }),
    subjectUserId: text("subjectUserId").references(() => users.id, {
      onDelete: "set null",
    }),
    actor: text("actor").notNull(),
    type: text("type").notNull(),
    detail: text("detail"),
  },
  (event) => [
    index("event_request_at").on(event.requestId, event.at),
    index("event_at").on(event.at),
  ],
);
