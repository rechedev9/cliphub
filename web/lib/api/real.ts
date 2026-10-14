import type { ApiClient } from './client.ts';
import { pinRenderRevision, renderRevisionFromPrefix } from './render-revision.ts';
import {
  applyMusicChoice,
  musicChoicesEqual,
  titleWithMusicSuffix,
  type MusicChoice,
} from './reel-music.ts';
import type { Match, Play, Song, Video, FeedItem, RenderMode, DemoPlayer, Preset, EditConfig, CaptureReadiness, CaptureTool, CaptureStatus, RosterMatch, ScannedDemo, SeriesDemo, JobStatusView } from './types.ts';
import { JOB_WAIT_TIMEOUT_CODE, MATCH_STATUS_FAILED, PLAN_READY_STATUSES, ROSTER_READY_STATUSES, SCAN_PENDING_STATUSES } from './types.ts';
import { planToMatch, planToPlays, type KillPlan } from './map.ts';
import { MISMATCH_REDRIVE_FAILURE_REASON, parseFailureReason } from './failure-reason.ts';
import { FULL_DEMO_PLANNER_VERSION } from '../full-demo-plan.ts';
import { customHudTheme } from '../custom-hud.ts';
import { canHaveRenderState, decideReelReconcile, isDurableAdmissionFailure, retryReelAction, shouldReconcileVideoStatus, viewForJobGone, viewForReadyWithoutVideo, viewForRecordAdmission, viewForRenderAdmission, type RedrivenRevision, type ReelAction, type ReelView, type RenderStatus } from './reel-reconcile.ts';
import { loadReelIntents, saveReelIntents, DEFAULT_VARIANT, DEFAULT_EDIT_CONFIG, type ReelIntent } from './reel-store.ts';
import { buildEditRequest, editConfigsEqual } from './edit-request.ts';
import { reelIdentity, shouldReuseReelIntent, fullDemoIntentConflict } from './reel-identity.ts';
import {
  applyEffectiveRenderMusic,
  clearVideoArtifactUrls,
  hydrateVideoFromIntent,
  parseEffectiveEditConfig,
  parseEffectiveRenderMusic,
  type EffectiveRenderMusic,
} from './render-hydration.ts';
import { dataPlane, type DataPlane } from './dataplane.ts';
import { parsePublishAssistant, type PublishAssistant } from './publish-assistant.ts';
import {
  listableJobs,
  planReadyJobs,
  summarizeSeries,
  enrichmentFromSummary,
  jobToMatch,
  type IndexedJob,
  type SeriesSummary,
} from './jobs-index.ts';
import { reconcileReels } from './reconcile-batch.ts';
import { recoveredVideo, recoveryProbes, type RecoveredVideo } from './recovered-renders.ts';
import { ifNoneMatchInit, readConditionalJSON, type ConditionalJSONCache } from './conditional-json.ts';
import { parseCaptureProgress } from '../capture-progress.ts';
import { playsSelectionLabel } from '../format.ts';
import { constrainEditConfig, isLandscapeRecap } from '../reel-brief.ts';

function toRosterMatch(m: RosterMatchResponse | undefined): RosterMatch | undefined {
  if (!m) return undefined;
  return { map: m.map, scoreCt: m.score_ct, scoreT: m.score_t, rounds: m.rounds };
}
