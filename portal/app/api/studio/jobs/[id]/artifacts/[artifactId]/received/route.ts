import { markArtifactReceived, readyArtifact } from "@/lib/artifact-store";
import { jsonError, notFound } from "@/lib/http";
import { findUserJob } from "@/lib/job-store";
import { resolveUser } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string; artifactId: string }> },
) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const { id, artifactId } = await params;
  const job = await findUserJob(resolved.user.userId, id);
  if (!job) return notFound();
  const artifact = await readyArtifact(job.id, artifactId);
  if (!artifact) return jsonError(410, "expired");
  await markArtifactReceived(artifact, Date.now());
  return Response.json({ ok: true });
}
