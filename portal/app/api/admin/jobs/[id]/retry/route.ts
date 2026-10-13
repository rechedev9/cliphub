import { invalidState, jsonError, notFound } from "@/lib/http";
import { retryJob } from "@/lib/queue-store";
import { resolveAdmin } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const { id } = await params;
  const result = await retryJob({ jobId: id, adminUserId: admin.user, now: Date.now() });
  if (result === "not_found") return notFound();
  if (result === "invalid_state") return invalidState();
  if (result === "demo_gone") return jsonError(409, "demo_gone");
  return Response.json({ ok: true });
}
