/**
 * Projection for one row of the orchestrator's batched reconciliation answer.
 *
 * It lives here rather than beside the route because the route module imports
 * `next/server`, which the unit runner cannot resolve; keeping the pure half in
 * `lib/` is what makes it testable.
 */

type GenerateRunUpstream = { variant?: string; active?: boolean; failure_reason?: string; failure_code?: string; failed_at?: string };
type GenerateRunBody = { variant: string; active: boolean; failure_reason?: string; failure_code?: string; failed_at?: string };

export type JobStatusUpstream = {
  status: string;
  failure_reason?: string;
  failure_code?: string;
  progress?: RawCaptureProgress;
  generate?: GenerateRunUpstream | null;
};

type JobStatusBody = {
  status: string;
  failure_reason?: string;
  failure_code?: string;
  progress?: CaptureProgress;
  generate?: GenerateRunBody;
};

export type BatchStatusUpstreamItem = {
  job_id: string;
  variant: string;
  job?: JobStatusUpstream | null;
  render?: unknown;
  error?: { code?: string; message?: string } | null;
};

export type BatchStatusItem = {
  job_id: string;
  variant: string;
  job: JobStatusBody | null;
  render: unknown;
  error?: { code?: string; message?: string };
};

type CaptureProgress = { done: number; total: number; percent?: number; stage?: string };
type RawCaptureProgress = { done?: number; total?: number; percent?: number; stage?: string } | undefined;

/**
 * Narrows one upstream row to the fields the browser is allowed to see.
 *
 * The error field is load-bearing and must never be dropped. A row the
 * orchestrator could not read carries an explicit error and no halves; without
 * it the row arrives as job:null + render:null, which reconcileOne reads as
 * "job gone" (404 strikes, then the unrecoverable latch) and deriveReelView
 * turns into a queued record intent that auto-fires an unrequested capture.
 */
export function projectBatchStatusItem(
  item: BatchStatusUpstreamItem,
  parseCaptureProgress: (raw: RawCaptureProgress) => CaptureProgress | undefined,
): BatchStatusItem {
  const out: BatchStatusItem = {
    job_id: item.job_id,
    variant: item.variant,
    job: null,
    render: item.render ?? null,
  };
  if (item.error) out.error = item.error;
  if (item.job) out.job = projectJobStatus(item.job, parseCaptureProgress);
  return out;
}

export function projectJobStatus(
  job: JobStatusUpstream,
  parseCaptureProgress: (raw: RawCaptureProgress) => CaptureProgress | undefined,
): JobStatusBody {
  const out: JobStatusBody = { status: job.status };
  if (job.failure_reason) out.failure_reason = job.failure_reason;
  if (job.failure_code) out.failure_code = job.failure_code;
  const progress = parseCaptureProgress(job.progress);
  if (progress) out.progress = progress;
  const generate = projectGenerateRun(job.generate);
  if (generate) out.generate = generate;
  return out;
}

function projectGenerateRun(run: GenerateRunUpstream | null | undefined): GenerateRunBody | undefined {
  if (!run?.variant) return undefined;
  const out: GenerateRunBody = { variant: run.variant, active: run.active === true };
  if (run.failure_reason) out.failure_reason = run.failure_reason;
  if (run.failure_code) out.failure_code = run.failure_code;
  if (run.failed_at) out.failed_at = run.failed_at;
  return out;
}
