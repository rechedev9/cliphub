import { numberParam } from "@/lib/http";
import { resolveAdmin } from "@/lib/user-context";
import { adminJobPage } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const query = new URL(request.url).searchParams;
  const page = await adminJobPage({
    status: query.get("status") || null,
    kind: query.get("kind") || null,
    userId: query.get("userId") || null,
    before: numberParam(query.get("before")),
    limit: numberParam(query.get("limit")),
    now: Date.now(),
  });
  return Response.json(page);
}
