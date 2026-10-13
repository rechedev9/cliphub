import { jsonError, leaseLost } from "@/lib/http";
import { completeJob } from "@/lib/queue-store";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id } = await params;
  const result = await completeJob({ worker, jobId: id, attempt, now: Date.now() });
  if (result === "lease_lost") return leaseLost();
  if (result === "no_artifacts") return jsonError(409, "no_artifacts");
  return Response.json({ ok: true });
}
