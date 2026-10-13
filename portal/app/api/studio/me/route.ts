import { unauthorized } from "@/lib/http";
import { resolveUser } from "@/lib/user-context";
import { studioMe } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const me = await studioMe(resolved.user.userId, Date.now());
  return me ? Response.json(me) : unauthorized();
}
