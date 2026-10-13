import { notFound } from "@/lib/http";
import { findUserJob } from "@/lib/job-store";
import { resolveUser } from "@/lib/user-context";
import { studioJobs } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const { id } = await params;
  const job = await findUserJob(resolved.user.userId, id);
  if (!job) return notFound();
  const [view] = await studioJobs([job], Date.now());
  return view ? Response.json(view) : notFound();
}
