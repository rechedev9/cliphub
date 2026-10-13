import { fileResponse } from "@/lib/file-response";
import { jsonError, leaseLost } from "@/lib/http";
import { leasedJob } from "@/lib/queue-store";
import { authenticateWorkerJobCall } from "@/lib/worker-auth";

export const runtime = "nodejs";

// The raw demo is served only here, and only to the worker that holds the job's lease.
export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const call = await authenticateWorkerJobCall(request);
  if (!call.ok) return call.response;
  const { worker, attempt } = call;
  const { id } = await params;
  const job = await leasedJob(worker, { jobId: id, attempt, statuses: ["running"] });
  if (!job) return leaseLost();
  if (!job.demoPath) return jsonError(410, "demo_gone");
  return fileResponse({
    request,
    path: job.demoPath,
    contentType: "application/octet-stream",
    headers: { "X-Demo-Sha256": job.demoSha256 ?? "" },
    missing: jsonError(410, "demo_gone"),
  });
}
