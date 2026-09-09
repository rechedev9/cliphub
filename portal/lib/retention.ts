import { rm } from "node:fs/promises";

import { and, eq, inArray, isNotNull, lt } from "drizzle-orm";

import { db } from "../db/client.ts";
import { requestArtifacts, requests } from "../db/schema.ts";

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;

// Statuses nothing will move out of on its own, so their files can expire.
export const TERMINAL_STATUSES = ["done", "failed", "rejected"] as const;

export interface RetentionConfig {
  // How long a delivered video stays downloadable.
  retentionDays: number;
  // How long a request that never finished uploading its demo lingers before
  // it stops counting against the submitter's quota.
  abandonedUploadHours: number;
  sweepIntervalMs: number;
}

export function loadRetentionConfig(
  env: Record<string, string | undefined> = process.env,
): RetentionConfig {
  return {
    retentionDays: Number(env.RETENTION_DAYS ?? 30),
    abandonedUploadHours: Number(env.ABANDONED_UPLOAD_HOURS ?? 24),
    sweepIntervalMs: Number(env.RETENTION_SWEEP_INTERVAL_MS ?? HOUR_MS),
  };
}

export interface RetentionCutoffs {
  terminalBefore: Date;
  awaitingDemoBefore: Date;
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
  };
}

export interface SweepResult {
  expiredArtifacts: number;
  expiredDeliveries: number;
  reapedAbandoned: number;
  removedDemos: number;
}

// Deletes what nobody can reach any more: videos past their retention window,
// requests abandoned before their demo ever arrived, and demo files left
// behind by requests that ended without the bridge ever claiming them.
// Request rows themselves are kept so the submitter still sees what happened
// to their demo — only the files go.
export async function runRetentionSweep(
  now: Date = new Date(),
  config: RetentionConfig = loadRetentionConfig(),
): Promise<SweepResult> {
  const cutoffs = retentionCutoffs(now, config);
  const result: SweepResult = {
    expiredArtifacts: 0,
    expiredDeliveries: 0,
    reapedAbandoned: 0,
    removedDemos: 0,
  };

  const expiredRequests = await db
    .select({ id: requests.id, finalVideoPath: requests.finalVideoPath })
    .from(requests)
    .where(
      and(
        inArray(requests.status, [...TERMINAL_STATUSES]),
        lt(requests.updatedAt, cutoffs.terminalBefore),
      ),
    );

  for (const request of expiredRequests) {
    const artifacts = await db
      .select()
      .from(requestArtifacts)
      .where(eq(requestArtifacts.requestId, request.id));

    for (const artifact of artifacts) {
      await rm(artifact.path, { force: true });
    }
    if (artifacts.length > 0) {
      await db
        .delete(requestArtifacts)
        .where(eq(requestArtifacts.requestId, request.id));
      result.expiredArtifacts += artifacts.length;
    }

    // finalVideoPath normally points at one of those artifact files, but
    // clear it unconditionally so "expired means no files left" holds even
    // for a delivered video that was never an artifact row.
    if (request.finalVideoPath) {
      await rm(request.finalVideoPath, { force: true });
      // Leave the row and its status alone — "delivered, then expired" is a
      // truer thing to show the submitter than the request disappearing —
      // and do not touch updatedAt, which still means "when this reached its
      // terminal state".
      await db
        .update(requests)
        .set({ finalVideoPath: null })
        .where(eq(requests.id, request.id));
      result.expiredDeliveries += 1;
    }
  }

  // A demo whose request ended without the bridge ever claiming it (rejected
  // while pending, say) still has the portal's copy on disk.
  const strandedDemos = await db
    .select({ id: requests.id, demoPath: requests.demoPath })
    .from(requests)
    .where(
      and(
        inArray(requests.status, [...TERMINAL_STATUSES]),
        isNotNull(requests.demoPath),
      ),
    );
  for (const request of strandedDemos) {
    if (!request.demoPath) continue;
    await rm(request.demoPath, { force: true });
    await db
      .update(requests)
      .set({ demoPath: null })
      .where(eq(requests.id, request.id));
    result.removedDemos += 1;
  }

  // Rows created by a submitter whose upload never completed. Nothing was
  // stored for them, and left alone they would count against that person's
  // active-request cap forever.
  const abandoned = await db
    .delete(requests)
    .where(
      and(
        eq(requests.status, "awaiting_demo"),
        lt(requests.createdAt, cutoffs.awaitingDemoBefore),
      ),
    )
    .returning({ id: requests.id });
  result.reapedAbandoned = abandoned.length;

  return result;
}

let sweeperStarted = false;

// Started from instrumentation.ts on server boot. A single-container app has
// nowhere better to put this; if in-process intervals ever prove unreliable
// across restarts, the same function can be driven by a host cron running
// `docker exec` instead.
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
