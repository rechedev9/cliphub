/** The job/fingerprint pair the server last acknowledged, or null before any PUT lands. */
export type AckedStreamPlanFingerprint = { jobId: string; fingerprint: string } | null;

/**
 * Whether an edit-plan autosave PUT is worth sending: the plan differs from the
 * last server-acknowledged revision for this job, or a PUT is still in flight
 * (its ack will describe a plan the user has already moved away from). Without
 * the first check autosave re-PUTs an unchanged plan on every open and bare
 * `stage` transition, rewriting the server artifact for nothing.
 */
export function shouldAutosaveStreamPlan(
  jobId: string,
  planFingerprint: string,
  acked: AckedStreamPlanFingerprint,
  inFlight: boolean,
): boolean {
  return inFlight || acked === null || acked.jobId !== jobId || acked.fingerprint !== planFingerprint;
}
