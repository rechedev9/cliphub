import { join } from "node:path";
import { randomUUID } from "node:crypto";
import { rm } from "node:fs/promises";
import type { ReadableStream as NodeWebReadableStream } from "node:stream/web";

import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { db } from "@/db/client";
import { requestArtifacts, requests } from "@/db/schema";
import { isBridgeAuthorized } from "@/lib/bridge-auth";
import { UploadTooLargeError, streamUploadToFile } from "@/lib/upload-stream";

export const runtime = "nodejs";

const UPLOAD_DIR = process.env.UPLOAD_DIR ?? "./data/uploads";
const MAX_ARTIFACT_BYTES = Number(
  process.env.MAX_ARTIFACT_BYTES ?? 1024 * 1024 * 1024,
);

// Receives one finished reel from the bridge. Reels arrive as candidates —
// the request stays "processing" until the owner promotes one in /admin —
// because a job can render several variants and several reels per variant,
// and only a human can say which one is the answer to the submitter's ask.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  if (!isBridgeAuthorized(request)) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const url = new URL(request.url);
  const variant = url.searchParams.get("variant")?.slice(0, 100);
  const name = url.searchParams.get("name")?.slice(0, 200);
  if (!variant || !name) {
    return NextResponse.json(
      { error: "variant and name query parameters are required" },
      { status: 400 },
    );
  }
  if (!request.body) {
    return NextResponse.json({ error: "missing body" }, { status: 400 });
  }

  const [existing] = await db.select().from(requests).where(eq(requests.id, id));
  if (!existing) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  if (existing.status !== "processing") {
    return NextResponse.json({ error: "request is not processing" }, { status: 409 });
  }

  // A distinct file per upload, so re-sending a reel never overwrites the
  // copy a previous row still points at until that row is replaced.
  const artifactId = randomUUID();
  const destPath = join(UPLOAD_DIR, id, "artifacts", `${artifactId}.mp4`);

  let bytesWritten: number;
  try {
    ({ bytesWritten } = await streamUploadToFile(
      request.body as unknown as NodeWebReadableStream<Uint8Array>,
      destPath,
      MAX_ARTIFACT_BYTES,
    ));
  } catch (err) {
    if (err instanceof UploadTooLargeError) {
      return NextResponse.json({ error: "artifact too large" }, { status: 413 });
    }
    throw err;
  }

  // The bridge remembers what it already sent, but if it ever loses that
  // state the same reel must replace its row rather than pile up beside it.
  const [previous] = await db
    .select()
    .from(requestArtifacts)
    .where(
      and(
        eq(requestArtifacts.requestId, id),
        eq(requestArtifacts.variant, variant),
        eq(requestArtifacts.name, name),
      ),
    );

  if (previous) {
    await db
      .update(requestArtifacts)
      .set({ path: destPath, sizeBytes: bytesWritten, uploadedAt: new Date() })
      .where(eq(requestArtifacts.id, previous.id));
    await rm(previous.path, { force: true });
  } else {
    await db.insert(requestArtifacts).values({
      id: artifactId,
      requestId: id,
      variant,
      name,
      path: destPath,
      sizeBytes: bytesWritten,
    });
  }

  return NextResponse.json({ ok: true, bytesWritten });
}
