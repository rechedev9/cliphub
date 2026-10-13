import { listDevices } from "@/lib/link-store";
import { resolveSessionUser } from "@/lib/user-context";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const resolved = await resolveSessionUser(request);
  if (!resolved.ok) return resolved.response;
  return Response.json({ devices: await listDevices(resolved.user.userId) });
}
