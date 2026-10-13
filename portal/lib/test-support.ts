import "./test-env.ts";

import { mkdir, writeFile } from "node:fs/promises";
import { dirname } from "node:path";

import { eq } from "drizzle-orm";

import { db } from "../db/client.ts";
import { runMigrations } from "../db/migrate.ts";
import {
  deviceLinks,
  devices,
  events,
  requestArtifacts,
  requests,
  userCloud,
  users,
  workers,
} from "../db/schema.ts";
import { demoPath } from "./demo-store.ts";
import { invalidateQueueSnapshot, type JobRow, type WorkerRow } from "./queue-store.ts";
import { hashToken, newWorkerToken } from "./tokens.ts";

export const NOW = 1_760_000_000_000;
export const MINUTE = 60_000;
export const DEMO_SHA = "a".repeat(64);

let migrated = false;

// A migrated, empty database before every test.
export async function resetDatabase(): Promise<void> {
  if (!migrated) {
    await runMigrations();
    migrated = true;
  }
  await db.delete(events);
  await db.delete(requestArtifacts);
  await db.delete(requests);
  await db.delete(devices);
  await db.delete(deviceLinks);
  await db.delete(userCloud);
  await db.delete(workers);
  await db.delete(users);
  invalidateQueueSnapshot();
}

export async function makeUser(name: string): Promise<string> {
  const [user] = await db
    .insert(users)
    .values({ name, email: `${name}@example.test` })
    .returning({ id: users.id });
  if (!user) throw new Error("user not created");
  return user.id;
}

// A worker that sent a heartbeat just now and is ready for work.
export async function makeWorker(name = "worker"): Promise<WorkerRow> {
  const [worker] = await db
    .insert(workers)
    .values({
      name,
      tokenHash: hashToken(newWorkerToken()),
      createdAt: NOW,
      lastSeenAt: NOW,
      state: "idle",
    })
    .returning();
  if (!worker) throw new Error("worker not created");
  return worker;
}

export function shortSpec(): Record<string, unknown> {
  return {
    version: 1,
    targetSteamId: "76561198000000001",
    rules: {},
    capture: { tickrate: 64, windows: [{ id: "seg-001", tickStart: 0, tickEnd: 6400 }] },
    generate: { preset: "viral-60-clean", music: null, segment_ids: ["seg-001"], edit: null },
    client: { studioVersion: "5.4.4" },
  };
}

export interface QueuedJobOptions {
  userId: string;
  enqueuedAt?: number;
  estimatedSeconds?: number;
  priorityBoost?: number;
  // false leaves the job pointing at a demo file that does not exist.
  withDemoFile?: boolean;
}

// A queued Short, as if its demo had just been uploaded.
export async function makeQueuedJob(options: QueuedJobOptions): Promise<string> {
  const path = demoPath(options.userId, DEMO_SHA);
  if (options.withDemoFile !== false) {
    await mkdir(dirname(path), { recursive: true });
    await writeFile(path, "PBDEMS2\0demo-bytes");
  }
  const enqueuedAt = options.enqueuedAt ?? NOW;
  const [job] = await db
    .insert(requests)
    .values({
      userId: options.userId,
      status: "queued",
      kind: "short",
      title: "R3 4k",
      spec: JSON.stringify(shortSpec()),
      demoPath: path,
      demoSha256: DEMO_SHA,
      demoSizeBytes: 18,
      demoOriginalName: "match.dem",
      estCaptureSeconds: 102,
      estimatedSeconds: options.estimatedSeconds ?? 480,
      enqueuedAt,
      priorityBoost: options.priorityBoost ?? 0,
      createdAt: new Date(enqueuedAt),
      updatedAt: new Date(enqueuedAt),
    })
    .returning({ id: requests.id });
  if (!job) throw new Error("job not created");
  return job.id;
}

export async function getJob(id: string): Promise<JobRow> {
  const [job] = await db.select().from(requests).where(eq(requests.id, id));
  if (!job) throw new Error(`job ${id} not found`);
  return job;
}

export async function getWorker(id: string): Promise<WorkerRow> {
  const [worker] = await db.select().from(workers).where(eq(workers.id, id));
  if (!worker) throw new Error(`worker ${id} not found`);
  return worker;
}

export async function eventTypes(requestId: string): Promise<string[]> {
  const rows = await db
    .select({ type: events.type })
    .from(events)
    .where(eq(events.requestId, requestId))
    .orderBy(events.at);
  return rows.map((row) => row.type);
}
