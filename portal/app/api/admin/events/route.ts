import { numberParam } from "@/lib/http";
import { resolveAdmin } from "@/lib/user-context";
import { adminEventLog } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const limit = numberParam(new URL(request.url).searchParams.get("limit"));
  return Response.json({ events: await adminEventLog(limit) });
}
