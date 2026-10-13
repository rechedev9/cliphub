import { failureResponse } from "@/lib/http";
import { attachDemo } from "@/lib/job-store";
import { resolveUser } from "@/lib/user-context";

export const runtime = "nodejs";

// A raw body, not multipart: it is streamed to disk without ever being buffered.
export async function PUT(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const { id } = await params;
  const result = await attachDemo({ user: resolved.user, jobId: id, request });
  if (!result.ok) return failureResponse(result);
  return Response.json({ ok: true, status: "queued" });
}
