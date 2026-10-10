import { isCloudAccess } from "@/lib/job-types";
import { resolveAdmin } from "@/lib/user-context";
import { adminUsers } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const query = new URL(request.url).searchParams;
  const access = query.get("access");
  const users = await adminUsers({
    query: query.get("query") ?? "",
    access: isCloudAccess(access) ? access : null,
    now: Date.now(),
  });
  return Response.json({ users });
}
