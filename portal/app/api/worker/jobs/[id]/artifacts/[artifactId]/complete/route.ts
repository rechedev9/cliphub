import { completeArtifact } from "@/lib/artifact-store";
import { failureResponse } from "@/lib/http";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string; artifactId: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id, artifactId } = await params;
  const result = await completeArtifact({
    worker,
    jobId: id,
    attempt,
    artifactId,
    now: Date.now(),
  });
  return result.ok ? Response.json(result) : failureResponse(result);
}
