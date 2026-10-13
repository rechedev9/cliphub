import { badRequest, leaseLost, readJson } from "@/lib/http";
import { enterUploading } from "@/lib/queue-store";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";
import { parsePhase } from "@/lib/worker-protocol";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id } = await params;
  const body = parsePhase(await readJson(request));
  if (!body) return badRequest();
  const entered = await enterUploading({ worker, jobId: id, attempt, body, now: Date.now() });
  return entered ? Response.json({ ok: true }) : leaseLost();
}
