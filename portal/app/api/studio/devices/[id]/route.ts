import { notFound } from "@/lib/http";
import { revokeDevice } from "@/lib/link-store";
import { resolveSessionUser } from "@/lib/user-context";

export const runtime = "nodejs";

export async function DELETE(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const resolved = await resolveSessionUser(request);
  if (!resolved.ok) return resolved.response;
  const { id } = await params;
  const revoked = await revokeDevice({
    deviceId: id,
    userId: resolved.user.userId,
    now: Date.now(),
  });
  return revoked ? new Response(null, { status: 204 }) : notFound();
}
