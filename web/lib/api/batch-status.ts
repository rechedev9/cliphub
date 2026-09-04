/**
 * Projection for one row of the orchestrator's batched reconciliation answer.
 *
 * It lives here rather than beside the route because the route module imports
 * `next/server`, which the unit runner cannot resolve; keeping the pure half in
 * `lib/` is what makes it testable.
 */

export type BatchStatusUpstreamItem = {
  job_id: string;
  variant: string;
  job?: {
    status: string;
    failure_reason?: string;
    failure_code?: string;
    progress?: { done?: number; total?: number; percent?: number };
  } | null;
  render?: unknown;
  error?: { code?: string; message?: string } | null;
};

export type BatchStatusItem = {
  job_id: string;
  variant: string;
  job: {
    status: string;
    failure_reason?: string;
    failure_code?: string;
    progress?: { done: number; total: number; percent?: number };
  } | null;
  render: unknown;
  error?: { code?: string; message?: string };
};

type CaptureProgress = { done: number; total: number; percent?: number };
type RawCaptureProgress = { done?: number; total?: number; percent?: number } | undefined;

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
  if (item.job) {
    out.job = { status: item.job.status };
    if (item.job.failure_reason) out.job.failure_reason = item.job.failure_reason;
    if (item.job.failure_code) out.job.failure_code = item.job.failure_code;
    const parsed = parseCaptureProgress(item.job.progress);
    if (parsed) out.job.progress = parsed;
  }
  return out;
}
