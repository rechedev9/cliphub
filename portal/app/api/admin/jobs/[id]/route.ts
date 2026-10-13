import { notFound } from "@/lib/http";
import { resolveAdmin } from "@/lib/user-context";
import { adminJobDetail } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const admin = await resolveAdmin(request);
  if (!admin.ok) return admin.response;
  const { id } = await params;
  const detail = await adminJobDetail(id, Date.now());
  return detail ? Response.json(detail) : notFound();
}
