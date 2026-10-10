import { writeArtifactPart } from "@/lib/artifact-store";
import { failureResponse } from "@/lib/http";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";

export const runtime = "nodejs";

export async function PUT(
  request: Request,
  { params }: { params: Promise<{ id: string; artifactId: string; n: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id, artifactId, n } = await params;
  const result = await writeArtifactPart({
    worker,
    jobId: id,
    attempt,
    artifactId,
    part: Number(n),
    request,
  });
  return result.ok ? Response.json(result) : failureResponse(result);
}
