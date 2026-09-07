import { eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isBridgeAuthorized } from "@/lib/bridge-auth";

export const runtime = "nodejs";

interface StatusBody {
  status?: string;
  reason?: string;
}

// Lets the bridge report a claimed request as failed (download error, local
// admission rejected the demo, etc.) so it stops showing as "processing"
// forever and the submitter sees why.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  if (!isBridgeAuthorized(request)) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const body = (await request.json().catch(() => null)) as StatusBody | null;
  if (body?.status !== "failed") {
    return NextResponse.json(
      { error: "status must be \"failed\"" },
      { status: 400 },
    );
  }

  await db
    .update(requests)
    .set({
      status: "failed",
      failureReason: body.reason?.slice(0, 500) || null,
      updatedAt: new Date(),
    })
    .where(eq(requests.id, id));

  return NextResponse.json({ ok: true });
}
