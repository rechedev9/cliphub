import { createReadStream } from "node:fs";
import { stat } from "node:fs/promises";
import { Readable } from "node:stream";

import { eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isBridgeAuthorized } from "@/lib/bridge-auth";

export const runtime = "nodejs";

// The raw .dem is served ONLY here, bearer-gated — never through any
// browser-facing route, not even to the submitter who uploaded it.
export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  if (!isBridgeAuthorized(request)) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const [existing] = await db.select().from(requests).where(eq(requests.id, id));
  if (!existing || !existing.demoPath) {
    return NextResponse.json({ error: "not found" }, { status: 404 });
  }
  // Only serve a demo the bridge itself just claimed — a valid token replayed
  // against an arbitrary id should not be able to fish for other demos.
  if (existing.status !== "processing") {
    return NextResponse.json({ error: "not claimed" }, { status: 409 });
  }

  const fileStat = await stat(existing.demoPath).catch(() => null);
  if (!fileStat) {
    return NextResponse.json({ error: "file missing" }, { status: 410 });
  }

  const nodeStream = createReadStream(existing.demoPath);
  const webStream = Readable.toWeb(
    nodeStream,
  ) as unknown as globalThis.ReadableStream<Uint8Array>;

  return new NextResponse(webStream, {
    headers: {
      "Content-Type": "application/octet-stream",
      "Content-Length": String(fileStat.size),
      "X-Filename": encodeURIComponent(existing.demoOriginalName ?? "demo.dem"),
    },
  });
}
