import { badRequest, readJson, unauthorized } from "@/lib/http";
import { recordHeartbeat } from "@/lib/queue-store";
import { authenticateWorker } from "@/lib/worker-auth";
import { parseHeartbeat } from "@/lib/worker-protocol";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const worker = await authenticateWorker(request);
  if (!worker) return unauthorized();
  const body = parseHeartbeat(await readJson(request));
  if (!body) return badRequest();
  return Response.json(await recordHeartbeat({ worker, body, now: Date.now() }));
}
