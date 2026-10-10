import { artifactResponse, readyArtifact } from "@/lib/artifact-store";
import { notFound } from "@/lib/http";
import { resolveAdmin } from "@/lib/user-context";

export const runtime = "nodejs";

// Streams a result to the operator's inline player.
export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string; artifactId: string }> },
) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const { id, artifactId } = await params;
  const artifact = await readyArtifact(id, artifactId);
  if (!artifact) return notFound();
  return artifactResponse({ request, artifact, attachment: false });
}
