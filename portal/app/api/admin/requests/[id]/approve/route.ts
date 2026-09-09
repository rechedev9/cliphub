import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";
import { isAdminUser } from "@/lib/admin";

export const runtime = "nodejs";

export async function POST(
  _request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await auth();
  if (!session?.user?.id || !(await isAdminUser(session.user.id))) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const updated = await db
    .update(requests)
    .set({ status: "approved", updatedAt: new Date() })
    .where(and(eq(requests.id, id), eq(requests.status, "pending")))
    .returning({ id: requests.id });

  if (updated.length === 0) {
    return NextResponse.json(
      { error: "request not found or not pending" },
      { status: 409 },
    );
  }

  return NextResponse.json({ ok: true });
}
