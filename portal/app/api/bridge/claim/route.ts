import { eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { db, rawClient } from "@/db/client";
import { users } from "@/db/schema";
import { isBridgeAuthorized } from "@/lib/bridge-auth";

export const runtime = "nodejs";

// A claim (status -> "processing") older than this with no local job yet is
// treated as abandoned — the bridge most likely crashed mid-download of a
// large .dem over a residential link — and becomes reclaimable again.
const STALE_CLAIM_MS = 15 * 60 * 1000;

interface ClaimedRow {
  id: string;
  note: string | null;
  demoOriginalName: string | null;
  userId: string;
}

// Atomically claims the single oldest approved-and-unclaimed request (or a
// stale abandoned claim). Uses raw SQL rather than the query builder: this
// specific "UPDATE ... WHERE id = (SELECT ... LIMIT 1) RETURNING" shape is
// what makes the claim atomic under concurrent bridge polls, and libsql
// doesn't support UPDATE ... LIMIT directly.
export async function POST(request: Request) {
  if (!isBridgeAuthorized(request)) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const now = Date.now();
  const staleThreshold = now - STALE_CLAIM_MS;

  const result = await rawClient.execute({
    sql: `
      update request
      set status = 'processing', claimedAt = ?
      where id = (
        select id from request
        where (status = 'approved' and localJobId is null)
           or (status = 'processing' and localJobId is null and (claimedAt is null or claimedAt < ?))
        order by createdAt asc
        limit 1
      )
      returning id, note, demoOriginalName, userId
    `,
    args: [now, staleThreshold],
  });

  const row = result.rows[0] as unknown as ClaimedRow | undefined;
  if (!row) {
    return NextResponse.json({ claimed: false });
  }

  const [submitter] = await db
    .select({ email: users.email, name: users.name })
    .from(users)
    .where(eq(users.id, row.userId));

  return NextResponse.json({
    claimed: true,
    id: row.id,
    note: row.note,
    demoOriginalName: row.demoOriginalName,
    submitterLabel: submitter?.name ?? submitter?.email ?? "unknown",
  });
}
