import { and, eq } from "drizzle-orm";
import { NextResponse } from "next/server";

import { auth } from "@/auth";
import { db } from "@/db/client";
import { requestArtifacts, requests } from "@/db/schema";
import { isAdminUser } from "@/lib/admin";

export const runtime = "nodejs";

interface CompleteBody {
  artifactId?: string;
}

// Promotes one uploaded candidate to the request's deliverable and hands it
// to the submitter. This is the second and final human gate: approving a
// request lets it into the queue, this says the result is the right one.
export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const session = await auth();
  if (!session?.user?.id || !(await isAdminUser(session.user.id))) {
    return NextResponse.json({ error: "forbidden" }, { status: 403 });
  }

  const { id } = await params;
  const body = (await request.json().catch(() => null)) as CompleteBody | null;
  const artifactId = body?.artifactId?.trim();
  if (!artifactId) {
    return NextResponse.json({ error: "artifactId is required" }, { status: 400 });
  }

  const [artifact] = await db
    .select()
    .from(requestArtifacts)
    .where(
      and(eq(requestArtifacts.id, artifactId), eq(requestArtifacts.requestId, id)),
    );
  if (!artifact) {
    return NextResponse.json({ error: "artifact not found" }, { status: 404 });
  }

  // Reel names carry no extension (the local pipeline appends .mp4 when it
  // builds the artifact key), so give the submitter a file their player will
  // actually open.
  const downloadName = artifact.name.endsWith(".mp4")
    ? `${artifact.variant}-${artifact.name}`
    : `${artifact.variant}-${artifact.name}.mp4`;

  const updated = await db
    .update(requests)
    .set({
      status: "done",
      finalVideoPath: artifact.path,
      finalVideoName: downloadName,
      failureReason: null,
      updatedAt: new Date(),
    })
    .where(eq(requests.id, id))
    .returning({ id: requests.id });

  if (updated.length === 0) {
    return NextResponse.json({ error: "request not found" }, { status: 404 });
  }

  return NextResponse.json({ ok: true });
}
