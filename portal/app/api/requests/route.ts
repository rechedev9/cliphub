import { and, eq, gte, inArray, sql } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";
import {
  ACTIVE_REQUEST_STATUSES,
  checkRequestLimits,
  loadRequestLimits,
} from "@/lib/request-limits";

export const runtime = "nodejs";

interface CreateRequestBody {
  note?: string;
}

// Step 1 of the two-step upload: create the row (status "awaiting_demo") so
// the client can then PUT the raw demo body to /api/requests/:id/demo without
// ever routing the file bytes through a buffered multipart parse.
export async function POST(request: Request) {
  const session = await auth();
  if (!session?.user?.id) {
    return NextResponse.json({ error: "unauthorized" }, { status: 401 });
  }

  const limits = loadRequestLimits();
  const oneDayAgo = new Date(Date.now() - 24 * 60 * 60 * 1000);
  const [[activeRow], [dailyRow]] = await Promise.all([
    db
      .select({ count: sql<number>`count(*)` })
      .from(requests)
      .where(
        and(
          eq(requests.userId, session.user.id),
          inArray(requests.status, ACTIVE_REQUEST_STATUSES),
        ),
      ),
    db
      .select({ count: sql<number>`count(*)` })
      .from(requests)
      .where(
        and(eq(requests.userId, session.user.id), gte(requests.createdAt, oneDayAgo)),
      ),
  ]);

  const violation = checkRequestLimits(
    { active: Number(activeRow?.count ?? 0), daily: Number(dailyRow?.count ?? 0) },
    limits,
  );
  if (violation === "active") {
    return NextResponse.json(
      { error: "tienes demasiadas peticiones activas; espera a que termine alguna" },
      { status: 429 },
    );
  }
  if (violation === "daily") {
    return NextResponse.json(
      { error: "has alcanzado el límite diario de peticiones" },
      { status: 429 },
    );
  }

  const body = (await request.json().catch(() => null)) as CreateRequestBody | null;
  const note = body?.note?.slice(0, 2000);

  const [created] = await db
    .insert(requests)
    .values({
      userId: session.user.id,
      note: note || null,
    })
    .returning({ id: requests.id });

  if (!created) {
    return NextResponse.json({ error: "could not create request" }, { status: 500 });
  }

  return NextResponse.json({ id: created.id }, { status: 201 });
}
