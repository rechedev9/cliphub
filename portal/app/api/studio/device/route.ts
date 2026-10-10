import { jsonError } from "@/lib/http";
import { revokeDevice } from "@/lib/link-store";
import { resolveUser } from "@/lib/user-context";

export const runtime = "nodejs";

// Studio unlinking itself: only the device token can revoke the device it belongs to.
export async function DELETE(request: Request) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const { userId, deviceId } = resolved.user;
  if (deviceId === null) return jsonError(403, "forbidden");
  await revokeDevice({ deviceId, userId, now: Date.now() });
  return new Response(null, { status: 204 });
}
