import { and, eq, isNull } from "drizzle-orm";

import { transact } from "../db/client.ts";
import { userCloud, users, workers } from "../db/schema.ts";
import { recordEvent } from "./events.ts";
import { isCloudAccess, isRecord, type CloudAccess } from "./job-types.ts";
import { invalidateQueueSnapshot, type WorkerRow } from "./queue-store.ts";
import { hashToken, newWorkerToken } from "./tokens.ts";
import { loadAccessDefault } from "./user-limits.ts";

const MAX_WORKER_NAME_CHARS = 60;
const MAX_REASON_CHARS = 300;
const MAX_NOTE_CHARS = 1000;
const MAX_ACTIVE_OVERRIDE = 100;
const MAX_DAILY_SECONDS_OVERRIDE = 7 * 24 * 60 * 60;

export type WorkerChange = "ok" | "not_found" | "invalid_state";

// A worker name as typed in the panel, or null when it is empty or too long.
export function parseWorkerName(input: unknown): string | null {
  if (typeof input !== "string") return null;
  const name = input.trim();
  return name.length > 0 && name.length <= MAX_WORKER_NAME_CHARS ? name : null;
}

export interface CreateWorkerInput {
  name: string;
  adminUserId: string;
  now: number;
}

// The token is returned once and only its hash is stored.
export async function createWorker(
  input: CreateWorkerInput,
): Promise<{ worker: WorkerRow; token: string }> {
  const { name, now } = input;
  const token = newWorkerToken();
  return transact(async (tx) => {
    const [worker] = await tx
      .insert(workers)
      .values({ name, tokenHash: hashToken(token), createdAt: now })
      .returning();
    if (!worker) throw new Error("could not create the worker");
    await recordEvent(
      {
        type: "worker_created",
        actor: `admin:${input.adminUserId}`,
        at: now,
        workerId: worker.id,
        detail: name,
      },
      tx,
    );
    return { worker, token };
  });
}

export interface WorkerActionInput {
  workerId: string;
  action: "pause" | "resume" | "revoke" | "rename";
  adminUserId: string;
  now: number;
  // The pause reason, or the new name.
  text?: string | null;
}

const WORKER_EVENTS = {
  pause: "worker_paused",
  resume: "worker_resumed",
  revoke: "worker_revoked",
  rename: null,
} as const;

// Pause, resume, revoke or rename. A revoked worker cannot be changed any more.
export async function changeWorker(input: WorkerActionInput): Promise<WorkerChange> {
  const { workerId, action, now } = input;
  const text = input.text ? input.text.slice(0, MAX_REASON_CHARS) : null;
  const changes = {
    pause: { paused: true, pausedBy: "admin", pauseReason: text },
    resume: { paused: false, pausedBy: null, pauseReason: null, consecutiveFailures: 0 },
    revoke: { revokedAt: now },
    rename: { name: text ?? "" },
  };

  const result = await transact(async (tx): Promise<WorkerChange> => {
    const [worker] = await tx.select().from(workers).where(eq(workers.id, workerId));
    if (!worker) return "not_found";
    if (worker.revokedAt !== null) return "invalid_state";
    await tx
      .update(workers)
      .set(changes[action])
      .where(and(eq(workers.id, workerId), isNull(workers.revokedAt)));
    const type = WORKER_EVENTS[action];
    if (type) {
      await recordEvent(
        { type, actor: `admin:${input.adminUserId}`, at: now, workerId, detail: text },
        tx,
      );
    }
    return "ok";
  });
  invalidateQueueSnapshot();
  return result;
}

export interface UserCloudPatch {
  access?: CloudAccess;
  maxActive?: number | null;
  dailySeconds?: number | null;
  note?: string | null;
}

function parseOverride(value: unknown, max: number): number | null | undefined {
  if (value === null) return null;
  if (typeof value === "number" && Number.isInteger(value) && value >= 1 && value <= max) {
    return value;
  }
  return undefined;
}

// Validates the body of POST /api/admin/users/:id. null means it is not acceptable.
export function parseUserCloudPatch(input: unknown): UserCloudPatch | null {
  if (!isRecord(input)) return null;
  const patch: UserCloudPatch = {};
  if ("access" in input) {
    if (!isCloudAccess(input.access)) return null;
    patch.access = input.access;
  }
  if ("maxActive" in input) {
    const value = parseOverride(input.maxActive, MAX_ACTIVE_OVERRIDE);
    if (value === undefined) return null;
    patch.maxActive = value;
  }
  if ("dailySeconds" in input) {
    const value = parseOverride(input.dailySeconds, MAX_DAILY_SECONDS_OVERRIDE);
    if (value === undefined) return null;
    patch.dailySeconds = value;
  }
  if ("note" in input) {
    if (input.note !== null && typeof input.note !== "string") return null;
    patch.note = input.note === null ? null : input.note.slice(0, MAX_NOTE_CHARS);
  }
  return patch;
}

export interface UpdateUserCloudInput {
  userId: string;
  patch: UserCloudPatch;
  adminUserId: string;
  now: number;
}

// Sets a user's access, limit overrides and note. False when the user does not exist.
export async function updateUserCloud(input: UpdateUserCloudInput): Promise<boolean> {
  const { userId, patch, now } = input;
  const updated = await transact(async (tx) => {
    const [user] = await tx.select({ id: users.id }).from(users).where(eq(users.id, userId));
    if (!user) return false;
    const [current] = await tx.select().from(userCloud).where(eq(userCloud.userId, userId));
    const next = {
      access: patch.access ?? current?.access ?? loadAccessDefault(),
      maxActive: patch.maxActive === undefined ? (current?.maxActive ?? null) : patch.maxActive,
      dailySeconds:
        patch.dailySeconds === undefined ? (current?.dailySeconds ?? null) : patch.dailySeconds,
      note: patch.note === undefined ? (current?.note ?? null) : patch.note,
      updatedAt: now,
    };
    await tx
      .insert(userCloud)
      .values({ userId, ...next })
      .onConflictDoUpdate({ target: userCloud.userId, set: next });

    const event = { actor: `admin:${input.adminUserId}`, at: now, subjectUserId: userId };
    if (patch.access !== undefined && patch.access !== current?.access) {
      await recordEvent({ ...event, type: "user_access_changed", detail: next.access }, tx);
    }
    const limitsChanged =
      next.maxActive !== (current?.maxActive ?? null) ||
      next.dailySeconds !== (current?.dailySeconds ?? null);
    if (limitsChanged) {
      await recordEvent(
        {
          ...event,
          type: "user_limits_changed",
          detail: `maxActive ${next.maxActive ?? "default"}, dailySeconds ${next.dailySeconds ?? "default"}`,
        },
        tx,
      );
    }
    return true;
  });
  invalidateQueueSnapshot();
  return updated;
}
