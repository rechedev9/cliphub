import { invalidState, notFound } from "@/lib/http";
import { findUserJob } from "@/lib/job-store";
import { cancelJob } from "@/lib/queue-store";
import { resolveUser } from "@/lib/user-context";

export const runtime = "nodejs";

export async function POST(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const { userId } = resolved.user;
  const { id } = await params;
  const job = await findUserJob(userId, id);
  if (!job) return notFound();
  const result = await cancelJob({
    jobId: job.id,
    by: "user",
    actorUserId: userId,
    now: Date.now(),
  });
  if (result === "not_found") return notFound();
  if (result === "invalid_state") return invalidState();
  return Response.json(result);
}
