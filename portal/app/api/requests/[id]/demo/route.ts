import { join } from "node:path";
import type { ReadableStream as NodeWebReadableStream } from "node:stream/web";

import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isDemoHeader } from "@/lib/demo-validate";
import {
  InvalidContentError,
  UploadTooLargeError,
  streamUploadToFile,
} from "@/lib/upload-stream";

export const runtime = "nodejs";

const UPLOAD_DIR = process.env.UPLOAD_DIR ?? "./data/uploads";
const MAX_DEMO_BYTES = Number(process.env.MAX_DEMO_BYTES ?? 700 * 1024 * 1024);

// Step 2 of the two-step upload: streams the raw .dem body straight to disk.
// Deliberately not multipart/form-data — a raw PUT body is the only way to
// avoid Next.js/Node buffering a 50-200MB file in memory before we see it.
export async function PUT(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const { id } = await params;
  const [existing] = await db
    .select()
    .from(requests)
    .where(and(eq(requests.id, id), eq(requests.userId, session.user.id)));

  if (!existing) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  if (existing.status !== "awaiting_demo") {
    return NextResponse.json(
      { error: "demo already uploaded for this request" },
      { status: 409 },
    );
  }
  if (!request.body) {
    return NextResponse.json({ error: "missing body" }, { status: 400 });
  }

  const rawFilename = request.headers.get("x-filename");
  const originalName = rawFilename
    ? decodeURIComponent(rawFilename).slice(0, 200)
    : "demo.dem";
  const destPath = join(UPLOAD_DIR, id, "demo.dem");

  try {
    const { bytesWritten, sha256 } = await streamUploadToFile(
      request.body as unknown as NodeWebReadableStream<Uint8Array>,
      destPath,
      MAX_DEMO_BYTES,
      { validateHeader: isDemoHeader },
    );
    await db
      .update(requests)
      .set({
        status: "pending",
        demoPath: destPath,
        demoSha256: sha256,
        demoOriginalName: originalName,
        updatedAt: new Date(),
      })
      .where(eq(requests.id, id));
    return NextResponse.json({ ok: true, bytesWritten });
  } catch (err) {
    if (err instanceof UploadTooLargeError) {
      return NextResponse.json({ error: "file too large" }, { status: 413 });
    }
    if (err instanceof InvalidContentError) {
      return NextResponse.json(
        { error: "uploaded file is not a CS2 demo" },
        { status: 400 },
      );
    }
    throw err;
  }
}
