import { createWorker, parseWorkerName } from "@/lib/admin-store";
import { badRequest, readJson } from "@/lib/http";
import { isRecord } from "@/lib/job-types";
import { resolveAdmin } from "@/lib/user-context";
import { adminWorkers } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  return Response.json({ workers: await adminWorkers(Date.now()) });
}

// The token is in this response and nowhere else: only its hash is stored.
export async function POST(request: Request) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const body = await readJson(request);
  const name = parseWorkerName(isRecord(body) ? body.name : undefined);
  if (name === null) return badRequest();
  const now = Date.now();
  const created = await createWorker({ name, adminUserId: admin.user, now });
  const [worker] = await adminWorkers(now, created.worker.id);
  return Response.json({ worker, token: created.token }, { status: 201 });
}
