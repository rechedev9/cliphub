import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requests } from "@/db/schema";

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
