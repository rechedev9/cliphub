import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import { Readable } from "node:stream";

import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requestArtifacts } from "@/db/schema";
import { isAdminUser } from "@/lib/admin";

export const runtime = "nodejs";

// Streams one candidate reel to the owner so they can watch it in /admin
// before deciding which one the submitter actually gets.
export async function GET(
  _request: Request,
  { params }: { params: Promise<{ id: string; artifactId: string }> },
) {
  const session = await auth();
  if (!session?.user?.id || !(await isAdminUser(session.user.id))) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id, artifactId } = await params;
  const [artifact] = await db
    .select()
    .from(requestArtifacts)
    .where(
      and(eq(requestArtifacts.id, artifactId), eq(requestArtifacts.requestId, id)),
    );
  if (!artifact) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }

  const fileStat = await stat(artifact.path).catch(() => null);
  if (!fileStat) {
    return NextResponse.json({ error: "file missing" }, { status: 410 });
  }

  const webStream = Readable.toWeb(
    createReadStream(artifact.path),
  ) as unknown as globalThis.ReadableStream<Uint8Array>;

  return new NextResponse(webStream, {
    headers: {
      "Content-Type": "video/mp4",
      "Content-Length": String(fileStat.size),
    },
  });
}
