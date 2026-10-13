import { randomUUID } from "node:crypto";
import { mkdir, rm } from "node:fs/promises";
import { dirname, join } from "node:path";

import { and, eq } from "drizzle-orm";

import { db, transact } from "../db/client.ts";
import { requestArtifacts } from "../db/schema.ts";
import {
  MAX_ARTIFACTS_PER_JOB,
  addReceivedPart,
  expectedPartLength,
  missingParts,
  parseReceivedParts,
  partCount,
} from "./artifact-parts.ts";
import { loadCloudConfig } from "./cloud-config.ts";
import { demoFileSize } from "./demo-store.ts";
import { fileResponse } from "./file-response.ts";
import { apiFailure, contentLength, jsonError, type ApiFailure } from "./http.ts";
import { isCloudKind } from "./job-types.ts";
import { leasedJob, type WorkerRow } from "./queue-store.ts";
import { PartLengthError, hashFile, readBytes, writePartAt } from "./upload-stream.ts";
import type { ArtifactInitBody } from "./worker-protocol.ts";

type ArtifactRow = typeof requestArtifacts.$inferSelect;

const UPLOADING = ["uploading"] as const;
const LEASE_LOST = apiFailure(409, "lease_lost");

function allParts(count: number): number[] {
  return Array.from({ length: count }, (_, index) => index + 1);
}

async function findArtifact(jobId: string, artifactId: string): Promise<ArtifactRow | null> {
  const [artifact] = await db
    .select()
    .from(requestArtifacts)
    .where(and(eq(requestArtifacts.id, artifactId), eq(requestArtifacts.requestId, jobId)));
  return artifact ?? null;
}

export interface ArtifactInitInput {
  worker: WorkerRow;
  jobId: string;
  attempt: number;
  body: ArtifactInitBody;
  now: number;
}

export type ArtifactInitResult =
  | { ok: true; artifactId: string; partSize: number; partCount: number; receivedParts: number[] }
  | ApiFailure;

// Opens (or resumes) the upload of one result file. Idempotent on (job, variant, name).
export async function initArtifact(input: ArtifactInitInput): Promise<ArtifactInitResult> {
  const { worker, attempt, body, now } = input;
  const config = loadCloudConfig();
  const job = await leasedJob(worker, { jobId: input.jobId, attempt, statuses: UPLOADING });
  if (!job || !isCloudKind(job.kind)) return LEASE_LOST;
  if (body.sizeBytes > config.maxArtifactBytes[job.kind]) {
    return apiFailure(413, "artifact_too_large");
  }

  const result = await transact(async (tx) => {
    const siblings = await tx
      .select()
      .from(requestArtifacts)
      .where(eq(requestArtifacts.requestId, job.id));
    const existing = siblings.find(
      (row) => row.variant === body.variant && row.name === body.name,
    );

    if (existing?.partSize && existing.sizeBytes === body.sizeBytes && existing.sha256 === body.sha256) {
      const count = partCount(existing.sizeBytes, existing.partSize);
      const receivedParts =
        existing.status === "ready" ? allParts(count) : parseReceivedParts(existing.receivedParts);
      return {
        ok: true as const,
        artifactId: existing.id,
        partSize: existing.partSize,
        partCount: count,
        receivedParts,
        stalePath: null,
      };
    }

    const fresh = {
      kind: body.kind,
      sizeBytes: body.sizeBytes,
      sha256: body.sha256,
      status: "uploading",
      partSize: config.artifactPartBytes,
      receivedParts: "[]",
      receivedAt: null,
      uploadedAt: new Date(now),
    };
    const count = partCount(body.sizeBytes, config.artifactPartBytes);
    if (existing) {
      // Same name, different content: the old bytes are worthless.
      await tx.update(requestArtifacts).set(fresh).where(eq(requestArtifacts.id, existing.id));
      return {
        ok: true as const,
        artifactId: existing.id,
        partSize: config.artifactPartBytes,
        partCount: count,
        receivedParts: [],
        stalePath: existing.path,
      };
    }
    if (siblings.length >= MAX_ARTIFACTS_PER_JOB) return apiFailure(409, "too_many_artifacts");

    // The path is built from server-generated ids only, never from the worker's file name.
    const artifactId = randomUUID();
    const path = join(config.uploadDir, job.id, "artifacts", `${artifactId}.bin`);
    await tx.insert(requestArtifacts).values({
      ...fresh,
      id: artifactId,
      requestId: job.id,
      variant: body.variant,
      name: body.name,
      path,
    });
    return {
      ok: true as const,
      artifactId,
      partSize: config.artifactPartBytes,
      partCount: count,
      receivedParts: [],
      stalePath: null,
    };
  });

  if (!result.ok) return result;
  const { stalePath, ...response } = result;
  if (stalePath) await rm(stalePath, { force: true });
  return response;
}

export interface ArtifactPartInput {
  worker: WorkerRow;
  jobId: string;
  attempt: number;
  artifactId: string;
  // 1-based.
  part: number;
  request: Request;
}

// Writes one part at its offset. Re-sending a part overwrites it.
export async function writeArtifactPart(
  input: ArtifactPartInput,
): Promise<{ ok: true; receivedParts: number[] } | ApiFailure> {
  const { worker, jobId, attempt, artifactId, part, request } = input;
  if (!(await leasedJob(worker, { jobId, attempt, statuses: UPLOADING }))) return LEASE_LOST;
  const artifact = await findArtifact(jobId, artifactId);
  if (!artifact) return apiFailure(404, "not_found");
  if (artifact.status !== "uploading" || !artifact.partSize) {
    return apiFailure(409, "invalid_state");
  }

  const { sizeBytes, partSize } = artifact;
  const expected = expectedPartLength({ sizeBytes, partSize, part });
  if (expected === null || contentLength(request) !== expected || !request.body) {
    return apiFailure(400, "bad_part_length");
  }

  await mkdir(dirname(artifact.path), { recursive: true });
  try {
    await writePartAt({
      body: request.body,
      path: artifact.path,
      offset: (part - 1) * partSize,
      length: expected,
    });
  } catch (err) {
    if (err instanceof PartLengthError) return apiFailure(400, "bad_part_length");
    throw err;
  }

  return transact(async (tx) => {
    const [current] = await tx
      .select()
      .from(requestArtifacts)
      .where(eq(requestArtifacts.id, artifact.id));
    // A re-init with other content while this part was in flight makes the part meaningless.
    if (!current || current.status !== "uploading" || current.sha256 !== artifact.sha256) {
      return apiFailure(409, "invalid_state");
    }
    const receivedParts = addReceivedPart(current.receivedParts, part);
    await tx
      .update(requestArtifacts)
      .set({ receivedParts: JSON.stringify(receivedParts) })
      .where(eq(requestArtifacts.id, artifact.id));
    return { ok: true as const, receivedParts };
  });
}

async function contentProblem(artifact: ArtifactRow): Promise<"sha256_mismatch" | "not_a_video" | null> {
  if ((await demoFileSize(artifact.path)) !== artifact.sizeBytes) return "sha256_mismatch";
  if ((await hashFile(artifact.path)) !== artifact.sha256) return "sha256_mismatch";
  if (artifact.kind !== "video") return null;
  // An mp4 starts with a box whose type, at bytes 4 to 7, is "ftyp".
  const boxType = await readBytes({ path: artifact.path, start: 4, length: 4 });
  return boxType.toString("latin1") === "ftyp" ? null : "not_a_video";
}

export interface ArtifactCompleteInput {
  worker: WorkerRow;
  jobId: string;
  attempt: number;
  artifactId: string;
  now: number;
}

// Verifies the assembled file and marks it ready for download.
export async function completeArtifact(
  input: ArtifactCompleteInput,
): Promise<{ ok: true } | ApiFailure> {
  const { worker, jobId, attempt, artifactId, now } = input;
  if (!(await leasedJob(worker, { jobId, attempt, statuses: UPLOADING }))) return LEASE_LOST;
  const artifact = await findArtifact(jobId, artifactId);
  if (!artifact) return apiFailure(404, "not_found");
  if (artifact.status === "ready") return { ok: true };
  if (!artifact.partSize) return apiFailure(409, "invalid_state");

  const count = partCount(artifact.sizeBytes, artifact.partSize);
  const missing = missingParts(count, parseReceivedParts(artifact.receivedParts));
  if (missing.length > 0) {
    return { ...apiFailure(409, "parts_missing"), extra: { missing } };
  }

  const problem = await contentProblem(artifact);
  const unchanged = and(
    eq(requestArtifacts.id, artifact.id),
    eq(requestArtifacts.status, "uploading"),
    eq(requestArtifacts.sizeBytes, artifact.sizeBytes),
  );
  if (problem) {
    // Nothing on disk can be trusted, so the worker starts this file over.
    await db.update(requestArtifacts).set({ receivedParts: "[]" }).where(unchanged);
    return apiFailure(422, problem);
  }
  const ready = await db
    .update(requestArtifacts)
    .set({ status: "ready", uploadedAt: new Date(now) })
    .where(unchanged)
    .returning({ id: requestArtifacts.id });
  return ready.length > 0 ? { ok: true } : apiFailure(409, "invalid_state");
}

// A finished result file of the job, or null. The caller has already checked who owns the job.
export async function readyArtifact(
  jobId: string,
  artifactId: string,
): Promise<ArtifactRow | null> {
  const artifact = await findArtifact(jobId, artifactId);
  return artifact?.status === "ready" ? artifact : null;
}

export interface ArtifactDownload {
  request: Request;
  artifact: ArtifactRow;
  // true sends it as a file to save; false lets a <video> element play it.
  attachment: boolean;
}

// Streams a result file with Range support. 410 once retention removed it.
export function artifactResponse(input: ArtifactDownload): Promise<Response> {
  const { artifact } = input;
  const headers: Record<string, string> = { "Cache-Control": "private, no-store" };
  if (artifact.sha256) headers.ETag = `"${artifact.sha256}"`;
  // The name passed ARTIFACT_NAME_PATTERN when it was stored, so it is safe inside the quotes.
  if (input.attachment) {
    headers["Content-Disposition"] = `attachment; filename="${artifact.name}"`;
  }
  return fileResponse({
    request: input.request,
    path: artifact.path,
    contentType: artifact.kind === "cover" ? "image/jpeg" : "video/mp4",
    headers,
    missing: jsonError(410, "expired"),
  });
}

// Studio confirms it has the file, which starts the 24 hour countdown to deletion.
export async function markArtifactReceived(artifact: ArtifactRow, now: number): Promise<void> {
  if (artifact.receivedAt !== null) return;
  await db
    .update(requestArtifacts)
    .set({ receivedAt: now })
    .where(eq(requestArtifacts.id, artifact.id));
}
