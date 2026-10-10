import { readdir, rm, rmdir, stat } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";

import { and, eq, inArray, isNotNull, lt, ne, notInArray } from "drizzle-orm";

import { db } from "../db/client.ts";
import { deviceLinks, events, requestArtifacts, requests } from "../db/schema.ts";
import { artifactExpiresAt } from "./artifact-parts.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import { demoUploadsInFlight, sweepDemos } from "./demo-store.ts";
import { TERMINAL_STATUSES } from "./job-types.ts";
import { invalidateQueueSnapshot } from "./queue-store.ts";

const MINUTE_MS = 60 * 1000;
const HOUR_MS = 60 * MINUTE_MS;
const DAY_MS = 24 * HOUR_MS;
// A link is only useful for ten minutes; the hour is slack for a slow last poll.
const EXPIRED_LINK_GRACE_MS = HOUR_MS;
const EVENT_RETENTION_MS = 90 * DAY_MS;
// A part can still be on its way to disk for a row that was just created.
const ORPHAN_ARTIFACT_GRACE_MS = HOUR_MS;

export interface RetentionConfig {
  // How long a video delivered by the old manual flow stays downloadable.
  retentionDays: number;
  // How long a cloud result stays downloadable when Studio never confirmed it.
  artifactRetentionDays: number;
  // How long a job that never received its demo lingers before it stops
  // counting against the submitter's active cap.
  abandonedUploadHours: number;
  sweepIntervalMs: number;
}

export function loadRetentionConfig(
  env: Record<string, string | undefined> = process.env,
): RetentionConfig {
  return {
    retentionDays: Number(env.RETENTION_DAYS ?? 30),
    // The same parser as the expiry date users are shown, so the two cannot disagree.
    artifactRetentionDays: loadCloudConfig(env).artifactRetentionDays,
    abandonedUploadHours: Number(env.ABANDONED_UPLOAD_HOURS ?? 24),
    sweepIntervalMs: Number(env.RETENTION_SWEEP_INTERVAL_MS ?? 10 * MINUTE_MS),
  };
}

export interface RetentionCutoffs {
  terminalBefore: Date;
  awaitingDemoBefore: Date;
  linksBefore: number;
  eventsBefore: number;
}

export function retentionCutoffs(
  now: Date,
  config: RetentionConfig,
): RetentionCutoffs {
  return {
    terminalBefore: new Date(now.getTime() - config.retentionDays * DAY_MS),
    awaitingDemoBefore: new Date(
      now.getTime() - config.abandonedUploadHours * HOUR_MS,
    ),
    linksBefore: now.getTime() - EXPIRED_LINK_GRACE_MS,
    eventsBefore: now.getTime() - EVENT_RETENTION_MS,
  };
}

export interface SweepResult {
  expiredArtifacts: number;
  orphanArtifacts: number;
  expiredDeliveries: number;
  reapedAbandoned: number;
  removedDemos: number;
  expiredLinks: number;
  prunedEvents: number;
}

async function removeArtifactFile(path: string): Promise<void> {
  await rm(path, { force: true });
  // Leave no empty <job>/artifacts folders behind; rmdir refuses when files remain.
  const folder = dirname(path);
  await rmdir(folder).catch(() => undefined);
  await rmdir(dirname(folder)).catch(() => undefined);
}

// Videos of the old manual flow: one retention window from the day the request ended.
async function sweepLegacyDeliveries(
  terminalBefore: Date,
  result: SweepResult,
): Promise<void> {
  const expiredRequests = await db
    .select({ id: requests.id, finalVideoPath: requests.finalVideoPath })
    .from(requests)
    .where(
      and(
        eq(requests.kind, "manual"),
        inArray(requests.status, [...TERMINAL_STATUSES]),
        lt(requests.updatedAt, terminalBefore),
      ),
    );

  for (const request of expiredRequests) {
    const artifacts = await db
      .delete(requestArtifacts)
      .where(eq(requestArtifacts.requestId, request.id))
      .returning({ path: requestArtifacts.path });
    for (const artifact of artifacts) await removeArtifactFile(artifact.path);
    result.expiredArtifacts += artifacts.length;

    // finalVideoPath normally points at one of those files, but clear it anyway
    // so "expired means no files left" also holds for a video that was never a row.
    if (request.finalVideoPath) {
      await rm(request.finalVideoPath, { force: true });
      await db
        .update(requests)
        .set({ finalVideoPath: null })
        .where(eq(requests.id, request.id));
      result.expiredDeliveries += 1;
    }
  }
}

// Cloud results: gone 24 hours after Studio confirmed them, or after the retention window.
async function sweepCloudArtifacts(
  options: { now: number; retentionDays: number },
  result: SweepResult,
): Promise<void> {
  const rows = await db
    .select({ artifact: requestArtifacts, jobStatus: requests.status })
    .from(requestArtifacts)
    .innerJoin(requests, eq(requests.id, requestArtifacts.requestId))
    .where(ne(requests.kind, "manual"));

  for (const { artifact, jobStatus } of rows) {
    const expired =
      artifact.status === "ready"
        ? artifactExpiresAt(artifact, options.retentionDays) <= options.now
        : TERMINAL_STATUSES.some((status) => status === jobStatus);
    if (!expired) continue;
    await db.delete(requestArtifacts).where(eq(requestArtifacts.id, artifact.id));
    await removeArtifactFile(artifact.path);
    result.expiredArtifacts += 1;
  }
}

// Result files no row points at: a part written after its job was requeued, or a
// restart between deleting the rows and deleting the files.
async function sweepOrphanArtifacts(now: number, result: SweepResult): Promise<void> {
  const root = loadCloudConfig().uploadDir;
  const rows = await db.select({ path: requestArtifacts.path }).from(requestArtifacts);
  const legacy = await db
    .select({ path: requests.finalVideoPath })
    .from(requests)
    .where(isNotNull(requests.finalVideoPath));
  const known = new Set<string>();
  for (const row of [...rows, ...legacy]) {
    if (row.path) known.add(resolve(row.path));
  }

  for (const entry of await readdir(root, { withFileTypes: true }).catch(() => [])) {
    if (!entry.isDirectory() || entry.name === "demos") continue;
    const folder = join(root, entry.name, "artifacts");
    for (const name of await readdir(folder).catch(() => [])) {
      const file = join(folder, name);
      // Only cloud results are named .bin; files of the old manual flow are left alone.
      if (!name.endsWith(".bin") || known.has(resolve(file))) continue;
      const fileStat = await stat(file).catch(() => null);
      if (!fileStat || now - fileStat.mtimeMs < ORPHAN_ARTIFACT_GRACE_MS) continue;
      await removeArtifactFile(file);
      result.orphanArtifacts += 1;
    }
  }
}

// Deletes what nobody can reach any more. Job rows are kept, so the submitter
// still sees what happened; only the files and stale bookkeeping go.
export async function runRetentionSweep(
  now: Date = new Date(),
  config: RetentionConfig = loadRetentionConfig(),
): Promise<SweepResult> {
  const cutoffs = retentionCutoffs(now, config);
  const result: SweepResult = {
    expiredArtifacts: 0,
    orphanArtifacts: 0,
    expiredDeliveries: 0,
    reapedAbandoned: 0,
    removedDemos: 0,
    expiredLinks: 0,
    prunedEvents: 0,
  };

  await sweepLegacyDeliveries(cutoffs.terminalBefore, result);
  await sweepCloudArtifacts(
    { now: now.getTime(), retentionDays: config.artifactRetentionDays },
    result,
  );
  await sweepOrphanArtifacts(now.getTime(), result);

  // Jobs whose demo never arrived would count against the active and queue caps forever.
  // One whose demo is arriving right now is not abandoned, however old it is.
  const receiving = demoUploadsInFlight().map((upload) => upload.jobId);
  const abandoned = await db
    .delete(requests)
    .where(
      and(
        eq(requests.status, "awaiting_demo"),
        lt(requests.createdAt, cutoffs.awaitingDemoBefore),
        receiving.length === 0 ? undefined : notInArray(requests.id, receiving),
      ),
    )
    .returning({ id: requests.id });
  result.reapedAbandoned = abandoned.length;
  if (abandoned.length > 0) invalidateQueueSnapshot();

  result.removedDemos = await sweepDemos(now.getTime());

  const links = await db
    .delete(deviceLinks)
    .where(lt(deviceLinks.expiresAt, cutoffs.linksBefore))
    .returning({ id: deviceLinks.id });
  result.expiredLinks = links.length;

  const pruned = await db
    .delete(events)
    .where(lt(events.at, cutoffs.eventsBefore))
    .returning({ id: events.id });
  result.prunedEvents = pruned.length;

  return result;
}

let sweeperStarted = false;

// Started from instrumentation on server boot. A single-container app has
// nowhere better to put this.
export function startRetentionSweeper(): void {
  if (sweeperStarted) return;
  sweeperStarted = true;

  const config = loadRetentionConfig();
  const sweep = () => {
    runRetentionSweep(new Date(), config)
      .then((result) => {
        if (Object.values(result).some((count) => count > 0)) {
          console.log("retention sweep:", result);
        }
      })
      .catch((err: unknown) => {
        console.error("retention sweep failed:", err);
      });
  };

  sweep();
  const timer = setInterval(sweep, config.sweepIntervalMs);
  // Never hold the process open just for the sweeper.
  timer.unref();
}
