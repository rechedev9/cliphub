import type { EditConfig, Video } from './types.ts';
import type { IndexedJob } from './jobs-index.ts';
import { canHaveRenderState } from './reel-reconcile.ts';
import { DEFAULT_EDIT_CONFIG, FULL_DEMO_REEL_SUFFIX } from './reel-store.ts';
import { pinRenderRevision, renderRevisionFromPrefix } from './render-revision.ts';
import { dataPlane } from './dataplane.ts';

/**
 * Finished renders the orchestrator still holds but no local reel intent points
 * at. Reel intents live in this browser's localStorage, so a cleared profile,
 * a reinstall or a different browser loses them while the MP4s stay on disk.
 * These videos are listed read-only: they are never reconciled or driven, so
 * adopting one can never trigger a capture or a re-render.
 */

/** Long-video render variant; its reel id has the fixed `full-demo` suffix. */
export const RECOVERY_FULL_DEMO_VARIANT = 'gameplay-pov-60';

/** Every variant a Studio build can have rendered for a demo job. */
export const RECOVERY_VARIANTS: readonly string[] = [
  RECOVERY_FULL_DEMO_VARIANT,
  'viral-60-clean',
  'clean-pov-60',
  'full-hud-60',
];

const SHORT_VARIANT_LABELS: Record<string, string> = {
  'viral-60-clean': 'Kill Feed',
  'clean-pov-60': 'Clean POV',
  'full-hud-60': 'Full HUD',
};

/** The render half of a status read, as RealApiClient parses it. */
export type RecoveryRender = {
  status: string;
  videoName?: string;
  coverName?: string;
  coverNames?: string[];
  artifactPrefix?: string;
  segmentIds?: string[];
  editConfig?: EditConfig;
};

export type RecoveredVideo = {
  video: Video;
  jobId: string;
  variant: string;
  videoName: string;
};

/** The reel id a render would carry if its intent still existed. */
export function recoveredVideoId(jobId: string, variant: string, segmentIds: readonly string[] | undefined): string | null {
  if (variant === RECOVERY_FULL_DEMO_VARIANT) return `${jobId}__${FULL_DEMO_REEL_SUFFIX}`;
  if (!segmentIds || segmentIds.length === 0) return null;
  return `${jobId}__${segmentIds.join('_')}`;
}

/** Jobs whose renders are worth one read: they may hold render state and were not read at this status yet. */
export function recoveryProbes(
  jobs: readonly IndexedJob[],
  probed: ReadonlyMap<string, string>,
): Array<{ jobId: string; variant: string }> {
  const out: Array<{ jobId: string; variant: string }> = [];
  for (const job of jobs) {
    if (!canHaveRenderState(job.status) || probed.get(job.jobId) === job.status) continue;
    for (const variant of RECOVERY_VARIANTS) out.push({ jobId: job.jobId, variant });
  }
  return out;
}

/** A read-only Video for a delivered render, or null when there is nothing to recover. */
export function recoveredVideo(
  job: IndexedJob,
  variant: string,
  render: RecoveryRender,
  knownIds: ReadonlySet<string>,
): RecoveredVideo | null {
  if (render.status !== 'ready' || !render.videoName) return null;
  const id = recoveredVideoId(job.jobId, variant, render.segmentIds);
  if (id === null || knownIds.has(id)) return null;

  const full = variant === RECOVERY_FULL_DEMO_VARIANT;
  const editConfig: EditConfig = full
    ? { ...DEFAULT_EDIT_CONFIG, ...render.editConfig, format: 'landscape-16x9', matchRecap: true }
    : { ...DEFAULT_EDIT_CONFIG, ...render.editConfig, format: 'short-9x16', matchRecap: false };
  const plays = render.segmentIds?.length ?? 0;
  const title = full
    ? 'Vídeo largo'
    : `${plays} ${plays === 1 ? 'jugada' : 'jugadas'} - ${SHORT_VARIANT_LABELS[variant] ?? variant}`;

  const dp = dataPlane();
  const revision = full ? renderRevisionFromPrefix(render.artifactPrefix, job.jobId, variant) : undefined;
  const cover = render.coverName ?? render.coverNames?.[0];
  const created = job.createdAt ? Date.parse(job.createdAt) : Number.NaN;
  const video: Video = {
    id,
    jobId: job.jobId,
    title,
    map: job.summary?.match?.map ?? 'Unknown',
    score: '',
    mode: 'clean',
    variant,
    editConfig,
    status: 'ready',
    recovered: true,
    createdAt: Number.isFinite(created) ? created : 0,
    downloadUrl: pinRenderRevision(dp.videoUrl(job.jobId, variant, render.videoName), revision),
  };
  if (job.summary?.target?.name) video.targetName = job.summary.target.name;
  if (revision) video.artifactRevision = revision;
  if (cover) video.thumbnailUrl = pinRenderRevision(dp.coverUrl(job.jobId, variant, cover), revision);
  return { video, jobId: job.jobId, variant, videoName: render.videoName };
}
