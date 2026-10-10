import { parseUserCloudPatch, updateUserCloud } from "@/lib/admin-store";
import { badRequest, notFound, readJson } from "@/lib/http";
import { resolveAdmin } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const { id } = await params;
  const patch = parseUserCloudPatch(await readJson(request));
  if (!patch) return badRequest();
  const updated = await updateUserCloud({
    userId: id,
    patch,
    adminUserId: admin.user,
    now: Date.now(),
  });
  return updated ? Response.json({ ok: true }) : notFound();
}
