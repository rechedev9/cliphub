import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import { Readable } from "node:stream";

import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";

export const runtime = "nodejs";

// The finished video is only ever served here, gated by an explicit
// (id AND userId) match — never by session alone — so one user can never
// reach another user's download by guessing/enumerating request ids.
export async function GET(
  _request: Request,
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
  if (existing.status !== "done" || !existing.finalVideoPath) {
    return NextResponse.json({ error: "not ready" }, { status: 409 });
  }

  const fileStat = await stat(existing.finalVideoPath).catch(() => null);
  if (!fileStat) {
    return NextResponse.json({ error: "file missing" }, { status: 410 });
  }

  const nodeStream = createReadStream(existing.finalVideoPath);
  const webStream = Readable.toWeb(
    nodeStream,
  ) as unknown as globalThis.ReadableStream<Uint8Array>;

  return new NextResponse(webStream, {
    headers: {
      "Content-Type": "video/mp4",
      "Content-Length": String(fileStat.size),
      "Content-Disposition": `attachment; filename="${existing.finalVideoName ?? "video.mp4"}"`,
    },
  });
}
