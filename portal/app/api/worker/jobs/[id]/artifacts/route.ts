import { initArtifact } from "@/lib/artifact-store";
import { badRequest, failureResponse, readJson } from "@/lib/http";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";
import { parseArtifactInit } from "@/lib/worker-protocol";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id } = await params;
  const body = parseArtifactInit(await readJson(request));
  if (!body) return badRequest();
  const result = await initArtifact({ worker, jobId: id, attempt, body, now: Date.now() });
  if (!result.ok) return failureResponse(result);
  return Response.json({
    artifactId: result.artifactId,
    partSize: result.partSize,
    partCount: result.partCount,
    receivedParts: result.receivedParts,
  });
}
