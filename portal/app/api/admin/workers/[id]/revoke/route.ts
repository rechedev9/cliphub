import { changeWorker } from "@/lib/admin-store";
import { invalidState, notFound } from "@/lib/http";
import { resolveAdmin } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const { id } = await params;
  const result = await changeWorker({
    workerId: id,
    action: "revoke",
    adminUserId: admin.user,
    now: Date.now(),
  });
  if (result === "not_found") return notFound();
  if (result === "invalid_state") return invalidState();
  return Response.json({ ok: true });
}
