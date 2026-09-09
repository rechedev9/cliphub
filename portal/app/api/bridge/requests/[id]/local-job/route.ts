import { rm } from "node:fs/promises";

import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isBridgeAuthorized } from "@/lib/bridge-auth";

export const runtime = "nodejs";

interface LocalJobBody {
  localJobId?: string;
}

// The bridge calls this right after admitting the demo locally. Idempotent
// by design (matches on status='processing' rather than localJobId IS NULL)
// so a retried report after a network blip does not fail just because the
// first attempt actually succeeded.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  if (!isBridgeAuthorized(request)) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const body = (await request.json().catch(() => null)) as LocalJobBody | null;
  const localJobId = body?.localJobId?.trim();
  if (!localJobId) {
    return NextResponse.json({ error: "localJobId is required" }, { status: 400 });
  }

  const updated = await db
    .update(requests)
    .set({ localJobId, updatedAt: new Date() })
    .where(and(eq(requests.id, id), eq(requests.status, "processing")))
    .returning({ id: requests.id, demoPath: requests.demoPath });

  const row = updated[0];
  if (!row) {
    return NextResponse.json(
      { error: "request not found or not in processing" },
      { status: 409 },
    );
  }

  // This report is the bridge confirming the demo is durably stored on the
  // owner's machine, so the portal's copy has no reader left. Dropping it
  // here rather than at retention time keeps one .dem, not two, and is what
  // stops a public upload queue from filling the VPS disk. Setting localJobId
  // also takes the request out of the stale-claim reclaim query, so nothing
  // will look for this file again.
  if (row.demoPath) {
    await rm(row.demoPath, { force: true });
    await db
      .update(requests)
      .set({ demoPath: null })
      .where(eq(requests.id, id));
  }

  return NextResponse.json({ ok: true });
}
