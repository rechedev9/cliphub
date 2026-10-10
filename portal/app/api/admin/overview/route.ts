import { resolveAdmin } from "@/lib/user-context";
import { adminOverview } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  return Response.json(await adminOverview(Date.now()));
}
