import {
  isMachineCode,
  isRetryCode,
  isTerminalCode,
  type FailureClass,
  type FailureCode,
} from "./job-types.ts";

const BREAKER_THRESHOLD = 3;
const PAUSE_DETAIL_CHARS = 300;
// The third machine fault of one job is treated as the job's own fault.
const MAX_MACHINE_REQUEUES = 2;

export function classOf(code: FailureCode): FailureClass {
  if (isRetryCode(code)) return "retry";
  if (isTerminalCode(code)) return "terminal";
  if (isMachineCode(code)) return "machine";
  return "cancel";
}

export interface FailureJob {
  attempt: number;
  maxAttempts: number;
  cancelRequestedAt: number | null;
  machineRequeues?: number;
  // Named in the pause reason, so the operator sees which job stopped the worker.
  id?: string;
  title?: string | null;
}

export interface FailureInput {
  job: FailureJob;
  code: FailureCode;
  // "lease" is the portal noticing an expired lease; "worker" is the worker reporting.
  source: "worker" | "lease";
  detail?: string | null;
  // The current counter of the worker that held the job.
  consecutiveFailures?: number;
}

export type FailureDecision =
  | { ok: false }
  | {
      ok: true;
      outcome: "requeued" | "failed" | "canceled";
      status: "queued" | "failed" | "canceled";
      attempt: number;
      machineRequeues: number;
      consecutiveFailures: number;
      // Not null when the worker must be auto-paused, with this reason.
      pauseReason: string | null;
    };

// "<code>: job <id> "<title>": <detail>"; the panel reads the job back out of it.
function pauseReasonFor(code: string, input: FailureInput): string {
  const detail = (input.detail ?? "").slice(0, PAUSE_DETAIL_CHARS);
  const { id, title } = input.job;
  if (!id) return `${code}: ${detail}`;
  return `${code}: job ${id} "${(title ?? "").replaceAll('"', "'")}": ${detail}`;
}

// Decides what a failure does to the job and to the worker (contracts section F).
export function decideFailure(input: FailureInput): FailureDecision {
  const { job, code, source } = input;
  const failureClass = classOf(code);
  const cancelPending = job.cancelRequestedAt !== null;
  let machineRequeues = job.machineRequeues ?? 0;
  let consecutiveFailures = input.consecutiveFailures ?? 0;
  let pauseReason: string | null = null;

  if (failureClass === "cancel") {
    if (!cancelPending) return { ok: false };
    return {
      ok: true,
      outcome: "canceled",
      status: "canceled",
      attempt: job.attempt,
      machineRequeues,
      consecutiveFailures,
      pauseReason,
    };
  }

  if (source === "worker" && (failureClass === "retry" || code === "job_failed")) {
    consecutiveFailures += 1;
    if (consecutiveFailures >= BREAKER_THRESHOLD) {
      pauseReason = pauseReasonFor("consecutive_failures", input);
      consecutiveFailures = 0;
    }
  }

  let attempt = job.attempt;
  let requeue = false;
  if (failureClass === "machine" && machineRequeues < MAX_MACHINE_REQUEUES) {
    // The machine is at fault, so the attempt is given back.
    attempt = Math.max(0, job.attempt - 1);
    machineRequeues += 1;
    requeue = true;
    pauseReason = pauseReasonFor(code, input);
  } else if (failureClass === "retry") {
    requeue = job.attempt < job.maxAttempts;
  }

  // Nobody is left to acknowledge the cancel of a job whose worker went silent.
  const canceled = cancelPending && (requeue || source === "lease");
  if (!requeue && !canceled) {
    return {
      ok: true,
      outcome: "failed",
      status: "failed",
      attempt,
      machineRequeues,
      consecutiveFailures,
      pauseReason,
    };
  }
  // A job someone asked to cancel is never put back in the queue.
  return {
    ok: true,
    outcome: canceled ? "canceled" : "requeued",
    status: canceled ? "canceled" : "queued",
    attempt,
    machineRequeues,
    consecutiveFailures,
    pauseReason,
  };
}
