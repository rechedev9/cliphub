import { SERVICE_UNAVAILABLE_CODE, type EditConfig, type VideoStatus, type CaptureProgress } from './types.ts';
import { requiresRecapture } from './failure-reason.ts';
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

/** Failed reels stay on the poll so a successful retry is not stuck on FALLO. */
export function shouldReconcileVideoStatus(status: VideoStatus | undefined): boolean {
  return status === undefined || status !== 'ready';
}

export type ReconcileInput = {
  jobStatus: string;
  jobFailureReason?: string;
  renderStatus: RenderStatus;
  renderFailureReason?: string;
  renderWarnings?: string[];
  renderArtifactPrefix?: string;
  /** Live capture progress from the job poll; meaningful only while recording. */
  captureProgress?: CaptureProgress;
  /** True after POST /record was accepted while the job may still read failed. */
  recordAdmitted?: boolean;
};

export type ReelView = {
  status: VideoStatus;
  action: ReelAction;
  /** Set only when status is 'failed' and the orchestrator supplied a reason. */
  failureReason?: string;
  /** Exact QA warnings that block publication while review is required. */
  warnings?: string[];
  /** Immutable revision that produced `warnings`; required for review CAS. */
  reviewArtifactPrefix?: string;
  /** Set only when status is 'recording' and the orchestrator reported progress. */
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

function failed(reason?: string): ReelView {
  return reason
    ? { status: 'failed', action: 'none', failureReason: reason }
    : { status: 'failed', action: 'none' };
}

const IN_FLIGHT_RECORD_CONFLICT = /^job is not ready to record \(status=recording\)$/;

/** HTTP POST /record result → reel view. Null keeps the current poll (202/503). */
export function viewForRecordAdmission(
  httpStatus: number,
  body: { error?: string; code?: string } = {},
): ReelView | null {
  if (httpStatus === 202 || httpStatus === 200) return null;
  if (httpStatus === 503 || body.code === SERVICE_UNAVAILABLE_CODE) return null;
  if (httpStatus === 404) return unrecoverableJobGoneView();
  if (httpStatus === 409 && IN_FLIGHT_RECORD_CONFLICT.test(body.error ?? '')) {
    return { status: 'recording', action: 'none' };
  }
  return failed(body.error || 'failed to start recording');
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

export type ReelReconcileDecision = {
  view: ReelView;
  adoptEffective: boolean;
};

export type DecideReelReconcileInput = ReconcileInput & {
  intentEdit: EditConfig;
  renderEdit?: EditConfig;
  intentMusic?: MusicChoice;
  renderMusic?: MusicChoice;
};

/** Shorts pack on this variant is not Full Demo; same capture + new mix re-renders. */
export function decideReelReconcile(input: DecideReelReconcileInput): ReelReconcileDecision {
  const ours = captureContractMatches(input.intentEdit, input.renderEdit);
  const readyOrReview = input.renderStatus === 'ready' || input.renderStatus === 'review_required';
  if (!ours && readyOrReview) {
    if (input.jobStatus === 'recording' || input.jobStatus === 'failed') {
      return { view: deriveReelView({ ...input, renderStatus: 'none' }), adoptEffective: false };
    }
    return { view: { status: 'queued', action: 'record' }, adoptEffective: false };
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
    return { view: { status: 'composing', action: 'render' }, adoptEffective: false };
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
    return failed(jobFailureReason);
  }
  if (renderStatus === 'failed') {
    // Stale capture: re-record. The worker clears this failed state after recapture.
    if (requiresRecapture(renderFailureReason)) {
      return { status: 'queued', action: 'record' };
    }
    return failed(renderFailureReason);
  }
  if (renderStatus === 'queued' || renderStatus === 'rendering') {
    return { status: 'composing', action: 'none' };
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
      return { status: 'composing', action: 'render' };
    case 'composing':
      return { status: 'composing', action: 'none' };
    default:
      // queued / scanning / scanned / parsing / unknown: not yet drivable as a reel.
      return { status: 'queued', action: 'none' };
  }
}
