import { changeWorker } from "@/lib/admin-store";
import { invalidState, notFound, readJson } from "@/lib/http";
import { isRecord } from "@/lib/job-types";
import { resolveAdmin } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const { id } = await params;
  const body = await readJson(request);
  const text = isRecord(body) && typeof body.reason === "string" ? body.reason : null;
  const result = await changeWorker({
    workerId: id,
    action: "pause",
    adminUserId: admin.user,
    now: Date.now(),
    text,
  });
  if (result === "not_found") return notFound();
  if (result === "invalid_state") return invalidState();
  return Response.json({ ok: true });
}
