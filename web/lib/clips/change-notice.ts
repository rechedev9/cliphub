import { CHANGE_NOT_APPLIED_REASON, parseFailureReason } from '../api/failure-reason.ts';
import type { Video } from '../api/types.ts';
import { OUTPUT_TYPE, isHighlightsVideo, outputType } from './hub.ts';

const CHANGE_FAILED_LEAD = 'Tu último cambio no se pudo grabar y este sigue siendo el vídeo anterior.';

export function changeNotice(video: Video): { message: string; retryCanHelp: boolean } | null {
  if (video.changePending !== true) return null;
  if (!video.failureReason) return { message: CHANGE_NOT_APPLIED_REASON, retryCanHelp: true };
  const failure = parseFailureReason(video.failureReason, {
    fullDemo: outputType(video) === OUTPUT_TYPE.full && !isHighlightsVideo(video),
    fullDemoPlannerVersion: video.editConfig?.fullDemo?.document.planner_version,
  });
  return { message: `${CHANGE_FAILED_LEAD} ${failure.message}`, retryCanHelp: failure.retryCanHelp };
}
