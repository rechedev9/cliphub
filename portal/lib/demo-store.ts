import { mkdir, readdir, rename, rm, stat } from "node:fs/promises";
import { dirname, join } from "node:path";

import { and, eq, gt, gte, inArray, isNotNull, isNull, ne, notInArray, or } from "drizzle-orm";

import { db, type Executor } from "../db/client.ts";
import { requests } from "../db/schema.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import { TERMINAL_STATUSES } from "./job-types.ts";

const HOUR_MS = 60 * 60 * 1000;
const ORPHAN_GRACE_MS = HOUR_MS;
const MAX_UPLOADS_PER_USER = 2;

// Demos are stored per user by content hash, so the same match is uploaded once.
export function demoPath(userId: string, sha256: string): string {
  return join(loadCloudConfig().uploadDir, "demos", userId, `${sha256}.dem`);
}

export function demoTempPath(userId: string, uploadId: string): string {
  return join(loadCloudConfig().uploadDir, "demos", userId, `${uploadId}.part`);
}

export interface DemoUpload {
  jobId: string;
  userId: string;
  // The declared length of the body, reserved on the volume until the upload ends.
  bytes: number;
}

declare global {
  // On globalThis so the routes and the retention sweeper, bundled apart, share one lock.
  var cliphubDemoLockTail: Promise<unknown> | undefined;
  // Demo uploads being received right now, by job id. Shared for the same reason.
  var cliphubDemoUploads: Map<string, DemoUpload> | undefined;
}

function uploadsInFlight(): Map<string, DemoUpload> {
  globalThis.cliphubDemoUploads ??= new Map();
  return globalThis.cliphubDemoUploads;
}

export function demoUploadsInFlight(): DemoUpload[] {
  return [...uploadsInFlight().values()];
}

export type DemoUploadSlot =
  | { ok: true; release: () => void }
  | { ok: false; code: "upload_in_progress" | "limit_uploads" };

// Reserves the upload before a byte is read: one per job, two per user. No await in here.
export function beginDemoUpload(upload: DemoUpload): DemoUploadSlot {
  const uploads = uploadsInFlight();
  if (uploads.has(upload.jobId)) return { ok: false, code: "upload_in_progress" };
  const ofUser = demoUploadsInFlight().filter((other) => other.userId === upload.userId);
  if (ofUser.length >= MAX_UPLOADS_PER_USER) return { ok: false, code: "limit_uploads" };
  uploads.set(upload.jobId, upload);
  return {
    ok: true,
    release: () => {
      if (uploads.get(upload.jobId) === upload) uploads.delete(upload.jobId);
    },
  };
}

export interface UserDemoBytesOptions {
  userId: string;
  // The demo about to be stored, counted once even when a job already holds it.
  incoming: { sha256: string; sizeBytes: number };
}

// Bytes of the distinct demos the user's jobs hold or still have to upload, plus the incoming one.
export async function userDemoBytes(
  executor: Executor,
  options: UserDemoBytesOptions,
): Promise<number> {
  const { userId, incoming } = options;
  const rows = await executor
    .select({
      id: requests.id,
      sha256: requests.demoSha256,
      sizeBytes: requests.demoSizeBytes,
    })
    .from(requests)
    .where(
      and(
        eq(requests.userId, userId),
        ne(requests.kind, "manual"),
        isNotNull(requests.demoSha256),
        or(isNotNull(requests.demoPath), eq(requests.status, "awaiting_demo")),
      ),
    );
  const uploads = uploadsInFlight();
  const sizes = new Map<string, number>([[incoming.sha256, incoming.sizeBytes]]);
  for (const row of rows) {
    if (row.sha256 === null) continue;
    // A job can declare one size when it is created and send another.
    const size = Math.max(row.sizeBytes ?? 0, uploads.get(row.id)?.bytes ?? 0);
    sizes.set(row.sha256, Math.max(sizes.get(row.sha256) ?? 0, size));
  }
  return [...sizes.values()].reduce((sum, size) => sum + size, 0);
}

// One at a time: reusing a stored demo for a new job must not race its deletion.
export function withDemoLock<T>(task: () => Promise<T>): Promise<T> {
  const previous = globalThis.cliphubDemoLockTail ?? Promise.resolve();
  const run = previous.then(task, task);
  globalThis.cliphubDemoLockTail = run.catch(() => undefined);
  return run;
}

export async function demoFileSize(path: string): Promise<number | null> {
  const fileStat = await stat(path).catch(() => null);
  return fileStat?.isFile() ? fileStat.size : null;
}

// Moves a verified upload to its content address. Call inside withDemoLock.
export async function adoptDemo(tempPath: string, finalPath: string): Promise<void> {
  if ((await demoFileSize(finalPath)) !== null) {
    await rm(tempPath, { force: true });
    return;
  }
  await mkdir(dirname(finalPath), { recursive: true });
  await rename(tempPath, finalPath);
}

// A demo is still needed by an unfinished job, or by a recent failure an operator may retry.
async function demoIsReferenced(path: string, now: number): Promise<boolean> {
  const retryWindowStart = now - loadCloudConfig().failedDemoRetentionHours * HOUR_MS;
  // A job its own user cancelled before it ever ran leaves nothing worth retrying.
  const worthRetrying = or(
    eq(requests.status, "failed"),
    gt(requests.attempt, 0),
    isNull(requests.canceledBy),
    ne(requests.canceledBy, "user"),
  );
  const [live] = await db
    .select({ id: requests.id })
    .from(requests)
    .where(
      and(
        eq(requests.demoPath, path),
        or(
          notInArray(requests.status, [...TERMINAL_STATUSES]),
          and(
            inArray(requests.status, ["failed", "canceled"]),
            gte(requests.finishedAt, retryWindowStart),
            worthRetrying,
          ),
        ),
      ),
    )
    .limit(1);
  return live !== undefined;
}

// Deletes the demo unless a job still needs it. True when it was deleted.
export async function releaseDemo(path: string, now: number): Promise<boolean> {
  return withDemoLock(async () => {
    if (await demoIsReferenced(path, now)) return false;
    await rm(path, { force: true });
    await db.update(requests).set({ demoPath: null }).where(eq(requests.demoPath, path));
    return true;
  });
}

async function listDemoFiles(): Promise<string[]> {
  const root = join(loadCloudConfig().uploadDir, "demos");
  const userDirs = await readdir(root, { withFileTypes: true }).catch(() => []);
  const files: string[] = [];
  for (const userDir of userDirs) {
    if (!userDir.isDirectory()) continue;
    const dir = join(root, userDir.name);
    for (const name of await readdir(dir).catch(() => [])) files.push(join(dir, name));
  }
  return files;
}

// Deletes demos no live job uses, and files no row points at (aborted uploads).
export async function sweepDemos(now: number): Promise<number> {
  const rows = await db
    .selectDistinct({ demoPath: requests.demoPath })
    .from(requests)
    .where(isNotNull(requests.demoPath));
  const known = new Set<string>();
  let removed = 0;
  for (const row of rows) {
    if (!row.demoPath) continue;
    if (await releaseDemo(row.demoPath, now)) removed += 1;
    else known.add(row.demoPath);
  }

  for (const file of await listDemoFiles()) {
    if (known.has(file)) continue;
    const fileStat = await stat(file).catch(() => null);
    if (!fileStat || now - fileStat.mtimeMs < ORPHAN_GRACE_MS) continue;
    if (await releaseDemo(file, now)) removed += 1;
  }
  return removed;
}
