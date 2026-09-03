import { GENERATE_WORK_ACTIVE_CODE, SERVICE_UNAVAILABLE_CODE, type EditConfig, type VideoStatus, type CaptureProgress } from './types.ts';
import { MISMATCH_REDRIVE_FAILURE_REASON, requiresRecapture } from './failure-reason.ts';
import { editConfigsEqual } from './edit-request.ts';
import { musicChoicesEqual, type MusicChoice } from './reel-music.ts';

export { requiresRecapture } from './failure-reason.ts';

/** Pure reel next-step: job + render status → UI view and one idempotent action. */

/** Render-variant lifecycle as the orchestrator reports it; 'none' = not started. */
export type RenderStatus =
  | 'none'
  | 'queued'
  | 'rendering'
  | 'ready'
  | 'review_required'
  | 'failed';

/** The one pipeline step to issue this tick (idempotent against server state). */
export type ReelAction = 'record' | 'render' | 'none';

/** Statuses that may already have render state. Earlier ones are a guaranteed 404. */
const RENDER_STATE_STATUSES = new Set<string>(['recorded', 'composing', 'composed', 'review_required', 'done', 'failed']);

/** True once a render GET can return something other than 404. */
export function canHaveRenderState(status: string): boolean {
  return RENDER_STATE_STATUSES.has(status);
}

/**
 * Failed reels stay on the poll so a successful retry is not stuck on FALLO.
 * A ready reel leaves the poll only once its MP4 URL is wired: the render GET
 * can report `ready` one tick before the artifact names, and nothing else
 * would ever fetch them.
 */
export function shouldReconcileVideoStatus(video: { status: VideoStatus; downloadUrl?: string } | undefined): boolean {
  if (video === undefined) return true;
  return video.status !== 'ready' || !video.downloadUrl;
}

export type ReconcileInput = {
  jobStatus: string;
  jobFailureReason?: string;
  jobFailureCode?: string;
  renderStatus: RenderStatus;
  renderFailureReason?: string;
  renderWarnings?: string[];
  renderArtifactPrefix?: string;
  /** Live job progress from the job poll; meaningful while recording or composing. */
  captureProgress?: CaptureProgress;
  /** True after POST /record was accepted while the job may still read failed. */
  recordAdmitted?: boolean;
};

export type ReelView = {
  status: VideoStatus;
  action: ReelAction;
  /** Set only when status is 'failed' and the orchestrator supplied a reason. */
  failureReason?: string;
  /** Stable orchestrator failure class; only set when the job (not the render) failed. */
  failureCode?: string;
  /** Exact QA warnings that block publication while review is required. */
  warnings?: string[];
  /** Immutable revision that produced `warnings`; required for review CAS. */
  reviewArtifactPrefix?: string;
  /** Set when the orchestrator reported progress during capture or editing. */
  captureProgress?: CaptureProgress;
  /** Job is gone; Retry cannot re-drive it. */
  unrecoverable?: true;
};

/** Status-poll 404: the orchestrator no longer has this job. */
const ORCHESTRATOR_JOB_GONE_REASON =
  'job no longer available (the local orchestrator may have restarted)';

/** Failed + unrecoverable view when the job 404s. Hide Retry. */
export function unrecoverableJobGoneView(): ReelView {
  return { ...failed(ORCHESTRATOR_JOB_GONE_REASON), unrecoverable: true };
}

/** Consecutive 404s before latching unrecoverable. One miss can be spurious. */
const JOB_GONE_LATCH_TICKS = 2;

/** Latch unrecoverable after enough 404s; below that leave the current view. */
export function viewForJobGone(consecutive404s: number): ReelView | null {
  return consecutive404s >= JOB_GONE_LATCH_TICKS ? unrecoverableJobGoneView() : null;
}

function failed(reason?: string, code?: string): ReelView {
  const view: ReelView = { status: 'failed', action: 'none' };
  if (reason) view.failureReason = reason;
  if (code) view.failureCode = code;
  return view;
}

const IN_FLIGHT_RECORD_CONFLICT = /^job is not ready to record \(status=recording\)$/;

type AdmissionBody = { error?: string; code?: string };

/** Shared POST admission mapping. Null keeps the current poll (202/503/work active). */
function viewForAdmission(httpStatus: number, body: AdmissionBody, fallbackReason: string): ReelView | null {
  if (httpStatus === 202 || httpStatus === 200) return null;
  if (httpStatus === 503 || body.code === SERVICE_UNAVAILABLE_CODE) return null;
  if (httpStatus === 409 && body.code === GENERATE_WORK_ACTIVE_CODE) return null;
  if (httpStatus === 404) return unrecoverableJobGoneView();
  return failed(body.error || fallbackReason);
}

/** HTTP POST /record (generate) result → reel view. */
export function viewForRecordAdmission(httpStatus: number, body: AdmissionBody = {}): ReelView | null {
  if (httpStatus === 409 && IN_FLIGHT_RECORD_CONFLICT.test(body.error ?? '')) {
    return { status: 'recording', action: 'none' };
  }
  return viewForAdmission(httpStatus, body, 'failed to start recording');
}

/** HTTP POST /renders/{variant} result → reel view. */
export function viewForRenderAdmission(httpStatus: number, body: AdmissionBody = {}): ReelView | null {
  return viewForAdmission(httpStatus, body, 'failed to start rendering');
}

/** A durable, retryable rejection (e.g. capture unconfigured): neither transient nor job-gone. */
export function isDurableAdmissionFailure(
  view: ReelView | null,
): view is ReelView & { status: 'failed'; failureReason: string } {
  return view !== null && view.status === 'failed' && view.unrecoverable !== true && view.failureReason !== undefined;
}

/** Retry drive: never re-POST record while capture is already running. */
export function retryReelAction(input: {
  jobStatus: string;
  renderStatus: RenderStatus;
  renderFailureReason?: string;
}): ReelAction {
  if (input.jobStatus === 'recording') return 'none';
  if (input.jobStatus === 'failed') return 'record';
  if (input.renderStatus === 'failed') {
    return requiresRecapture(input.renderFailureReason) ? 'record' : 'render';
  }
  return 'none';
}

/** Capture-time bits: a ready render that disagrees must not be this reel. */
function captureContractMatches(intentEdit: EditConfig, renderEdit: EditConfig | undefined): boolean {
  if (!renderEdit) return false;
  return intentEdit.matchRecap === renderEdit.matchRecap && intentEdit.nativeHud === renderEdit.nativeHud;
}

function selectionMatches(
  intentEdit: EditConfig,
  intentSegmentIds: readonly string[] | undefined,
  renderSegmentIds: readonly string[] | undefined,
): boolean {
  // A recap expands its empty UI selection to every round in the durable plan.
  if (intentEdit.matchRecap) return true;
  if (!intentSegmentIds || !renderSegmentIds || intentSegmentIds.length !== renderSegmentIds.length) return false;
  return intentSegmentIds.every((id, index) => id === renderSegmentIds[index]);
}

/** Render-time mix: same capture can still need a new encode. */
function renderDeliveryMatches(
  intentEdit: EditConfig,
  renderEdit: EditConfig | undefined,
  intentMusic: MusicChoice,
  renderMusic: MusicChoice | undefined,
): boolean {
  if (!renderEdit || !editConfigsEqual(intentEdit, renderEdit)) return false;
  if (!renderMusic) return !intentMusic.songId;
  return musicChoicesEqual(intentMusic, renderMusic);
}

/** The mismatching revision a reel already re-drove `record` from. */
export type RedrivenRevision = { artifactPrefix?: string };

/**
 * Next step for a ready/review render that disagrees with the intent: re-drive
 * once per explicit user action, wait while that revision is still current, and
 * fail once a different revision still mismatches (the backend ignored the intent).
 */
export type MismatchRedrive = 'drive' | 'wait' | 'fail';

export type ReelReconcileDecision = {
  view: ReelView;
  adoptEffective: boolean;
  /** Present only when the ready/review render mismatched the intent. */
  mismatchRedrive?: MismatchRedrive;
};

export type DecideReelReconcileInput = ReconcileInput & {
  intentEdit: EditConfig;
  renderEdit?: EditConfig;
  intentSegmentIds?: readonly string[];
  renderSegmentIds?: readonly string[];
  intentMusic?: MusicChoice;
  renderMusic?: MusicChoice;
  /** Revision this reel already re-drove from since the last explicit user action. */
  redrivenRevision?: RedrivenRevision;
};

function mismatchDecision(input: DecideReelReconcileInput): ReelReconcileDecision {
  const previous = input.redrivenRevision;
  if (previous === undefined) {
    return { view: { status: 'queued', action: 'record' }, adoptEffective: false, mismatchRedrive: 'drive' };
  }
  if (previous.artifactPrefix === input.renderArtifactPrefix) {
    return { view: { status: 'queued', action: 'none' }, adoptEffective: false, mismatchRedrive: 'wait' };
  }
  return { view: failed(MISMATCH_REDRIVE_FAILURE_REASON), adoptEffective: false, mismatchRedrive: 'fail' };
}

/** Reconcile a local reel intent with the durable variant and capture facts. */
export function decideReelReconcile(input: DecideReelReconcileInput): ReelReconcileDecision {
  const ours =
    captureContractMatches(input.intentEdit, input.renderEdit) &&
    selectionMatches(input.intentEdit, input.intentSegmentIds, input.renderSegmentIds);
  const readyOrReview = input.renderStatus === 'ready' || input.renderStatus === 'review_required';
  if (!ours && readyOrReview) {
    if (input.jobStatus === 'recording' || input.jobStatus === 'failed') {
      return { view: deriveReelView({ ...input, renderStatus: 'none' }), adoptEffective: false };
    }
    return mismatchDecision(input);
  }
  const delivery = renderDeliveryMatches(
    input.intentEdit,
    input.renderEdit,
    input.intentMusic ?? {},
    input.renderMusic,
  );
  if (ours && readyOrReview && !delivery) {
    if (input.jobStatus === 'recording' || input.jobStatus === 'failed') {
      return { view: deriveReelView({ ...input, renderStatus: 'none' }), adoptEffective: false };
    }
    // The single current recording may belong to another variant. Generate
    // validates it server-side and either reuses it or recaptures from the DEM.
    return mismatchDecision(input);
  }
  return {
    view: deriveReelView(input),
    adoptEffective: ours && delivery && readyOrReview,
  };
}

export function deriveReelView(input: ReconcileInput): ReelView {
  const {
    jobStatus,
    jobFailureReason,
    jobFailureCode,
    renderStatus,
    renderFailureReason,
    renderWarnings,
    renderArtifactPrefix,
    captureProgress,
    recordAdmitted,
  } = input;

  // A finished render is terminal even if the job later fails.
  if (renderStatus === 'ready') return { status: 'ready', action: 'none' };
  if (renderStatus === 'review_required') {
    return {
      status: 'review_required',
      action: 'none',
      ...(renderWarnings?.length ? { warnings: renderWarnings } : {}),
      ...(renderArtifactPrefix ? { reviewArtifactPrefix: renderArtifactPrefix } : {}),
    };
  }
  if (jobStatus === 'failed') {
    if (recordAdmitted) {
      return { status: 'recording', action: 'none' };
    }
    // Non-reusable capture: re-drive record instead of staying failed.
    if (requiresRecapture(jobFailureReason)) {
      return { status: 'queued', action: 'record' };
    }
    return failed(jobFailureReason, jobFailureCode);
  }
  if (renderStatus === 'failed') {
    // Stale capture: re-record. The worker clears this failed state after recapture.
    if (requiresRecapture(renderFailureReason)) {
      return { status: 'queued', action: 'record' };
    }
    return failed(renderFailureReason);
  }
  if (renderStatus === 'queued' || renderStatus === 'rendering') {
    return captureProgress
      ? { status: 'composing', action: 'none', captureProgress }
      : { status: 'composing', action: 'none' };
  }

  // renderStatus === 'none': decide the next step from the job's own progress.
  switch (jobStatus) {
    case 'recording':
      // Omit captureProgress when the poll has not reported segments yet.
      return captureProgress
        ? { status: 'recording', action: 'none', captureProgress }
        : { status: 'recording', action: 'none' };
    case 'parsed':
      return { status: 'queued', action: 'record' };
    case 'recorded':
    case 'composed':
    case 'done':
      // A job-level recorded state says only that some capture exists. Let the
      // generate endpoint validate the exact Short/Full Demo capture contract.
      return { status: 'queued', action: 'record' };
    case 'composing':
      return captureProgress
        ? { status: 'composing', action: 'none', captureProgress }
        : { status: 'composing', action: 'none' };
    default:
      // queued / scanning / scanned / parsing / unknown: not yet drivable as a reel.
      return { status: 'queued', action: 'none' };
  }
}
