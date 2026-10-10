import { badRequest, invalidState, leaseLost, readJson } from "@/lib/http";
import { failJob } from "@/lib/queue-store";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";
import { parseFail } from "@/lib/worker-protocol";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id } = await params;
  const body = parseFail(await readJson(request));
  if (!body) return badRequest();
  const result = await failJob({ worker, jobId: id, attempt, body, now: Date.now() });
  if (result === "lease_lost") return leaseLost();
  if (result === "invalid") return invalidState();
  return Response.json(result);
}
