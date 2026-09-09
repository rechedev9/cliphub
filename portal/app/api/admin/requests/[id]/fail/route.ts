import { eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isAdminUser } from "@/lib/admin";

export const runtime = "nodejs";

interface FailBody {
  reason?: string;
}

// Closes out a request the owner cannot deliver — the demo turned out to be
// unusable, the capture never worked. Distinct from /reject, which declines
// a request before it ever enters the queue.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await auth();
  if (!session?.user?.id || !(await isAdminUser(session.user.id))) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const body = (await request.json().catch(() => null)) as FailBody | null;
  const reason = body?.reason?.slice(0, 500);

  const updated = await db
    .update(requests)
    .set({
      status: "failed",
      failureReason: reason || "the operator could not complete this request",
      updatedAt: new Date(),
    })
    .where(eq(requests.id, id))
    .returning({ id: requests.id });

  if (updated.length === 0) {
    return NextResponse.json({ error: "request not found" }, { status: 404 });
  }

  return NextResponse.json({ ok: true });
}
