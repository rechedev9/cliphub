import { readJson, unauthorized } from "@/lib/http";
import { isRecord } from "@/lib/job-types";
import { claimNext } from "@/lib/queue-store";
import { authenticateWorker } from "@/lib/worker-auth";
import { parseKinds } from "@/lib/worker-protocol";

export const runtime = "nodejs";

export async function POST(request: Request) {
  const worker = await authenticateWorker(request);
  if (!worker) return unauthorized();
  const body = await readJson(request);
  const kinds = parseKinds(isRecord(body) ? body.kinds : undefined);
  return Response.json(await claimNext({ worker, kinds, now: Date.now() }));
}
