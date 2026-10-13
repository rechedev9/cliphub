import { failureResponse, readJson } from "@/lib/http";
import { createJob, listUserJobs } from "@/lib/job-store";
import { resolveUser } from "@/lib/user-context";
import { studioJobs } from "@/lib/views";

export const runtime = "nodejs";

export async function GET(request: Request) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const now = Date.now();
  const jobs = await listUserJobs(resolved.user.userId, now);
  return Response.json({ jobs: await studioJobs(jobs, now) });
}

export async function POST(request: Request) {
  const resolved = await resolveUser(request);
  if (!resolved.ok) return resolved.response;
  const result = await createJob({
    user: resolved.user,
    body: await readJson(request),
    now: Date.now(),
  });
  if (!result.ok) return failureResponse(result);
  return Response.json(
    { id: result.id, status: result.status, demoUpload: result.demoUpload },
    { status: 201 },
  );
}
