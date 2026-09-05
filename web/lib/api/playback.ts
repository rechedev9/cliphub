import type { Video } from './types.ts';
import type { StreamJob, StreamRenderedOutput } from './streams.ts';

export const PLAYBACK_SOURCE = {
  demo: 'demo',
  stream: 'stream',
} as const;

export const PLAYBACK_REVIEW = {
  ready: 'ready',
  pending: 'pending',
  stale: 'stale',
} as const;

export type PlaybackSource = (typeof PLAYBACK_SOURCE)[keyof typeof PLAYBACK_SOURCE];
export type PlaybackReview = (typeof PLAYBACK_REVIEW)[keyof typeof PLAYBACK_REVIEW];
export type PlaybackFormat = '9:16' | '16:9';

/** A browser-safe reference to one immutable rendered artifact. */
export type MediaPlaybackItem = {
  id: string;
  source: PlaybackSource;
  jobId: string;
  variant: string;
  artifactName: string;
  revision: string;
  title: string;
  format: PlaybackFormat;
  durationSeconds?: number;
  posterUrl?: string;
  playbackUrl: string;
  review: PlaybackReview;
  warnings: string[];
};

function artifactNameFromUrl(url: string): string | null {
  const path = url.split('?', 1)[0] ?? '';
  const value = path.split('/').filter(Boolean).at(-1);
  if (!value) return null;
  try {
    const decoded = decodeURIComponent(value);
    return decoded.length <= 255 && /^[^/\\]+$/.test(decoded) ? decoded : null;
  } catch {
    return null;
  }
}

const ARTIFACT_REVISION_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function demoArtifactRevision(prefix: string | undefined): string | null {
  const revision = prefix?.split(/[\\/]/).filter(Boolean).at(-1);
  return revision && ARTIFACT_REVISION_RE.test(revision) ? revision : null;
}

function legacyDemoRevision(prefix: string | undefined, video: Video, artifactName: string): string {
  if (prefix) {
    let first = 2_166_136_261;
    let second = 2_166_136_261;
    for (let index = 0; index < prefix.length; index += 1) {
      first = Math.imul(first ^ prefix.charCodeAt(index), 16_777_619);
      second = Math.imul(second ^ prefix.charCodeAt(prefix.length - index - 1), 16_777_619);
    }
    return `legacy:${(first >>> 0).toString(16)}${(second >>> 0).toString(16)}`;
  }
  return `legacy:${video.id}:${video.createdAt}:${artifactName}`;
}

export function demoPlaybackItem(video: Video): MediaPlaybackItem | null {
  if (!video.downloadUrl || !video.jobId) return null;
  const variant = video.variant ?? 'viral-60-clean';
  const artifactName = artifactNameFromUrl(video.downloadUrl);
  if (artifactName === null) return null;
  const artifactRevision = demoArtifactRevision(video.reviewArtifactPrefix);
  const immutableBase = artifactRevision === null
    ? null
    : `/api/demos/${encodeURIComponent(video.jobId)}/renders/${encodeURIComponent(variant)}/revisions/${artifactRevision}`;
  const coverName = video.thumbnailUrl ? artifactNameFromUrl(video.thumbnailUrl) : null;
  return {
    id: `demo:${video.id}`,
    source: PLAYBACK_SOURCE.demo,
    jobId: video.jobId,
    variant,
    artifactName,
    revision: artifactRevision ?? legacyDemoRevision(video.reviewArtifactPrefix, video, artifactName),
    title: video.title,
    format: video.editConfig?.format === 'landscape-16x9' ? '16:9' : '9:16',
    posterUrl: immutableBase !== null && coverName !== null
      ? `${immutableBase}/covers/${encodeURIComponent(coverName)}`
      : video.thumbnailUrl,
    playbackUrl: immutableBase === null
      ? video.downloadUrl
      : `${immutableBase}/videos/${encodeURIComponent(artifactName)}`,
    review: video.status === 'review_required' ? PLAYBACK_REVIEW.pending : PLAYBACK_REVIEW.ready,
    warnings: video.warnings ?? [],
  };
}

function browserStreamUrl(url: string): string | null {
  if (url.startsWith('/api/streams/')) return url;
  if (url.startsWith('/api/stream-jobs/')) return `/api/streams/${url.slice('/api/stream-jobs/'.length)}`;
  return null;
}

export function streamPlaybackItem(job: StreamJob, output: StreamRenderedOutput): MediaPlaybackItem | null {
  const playbackUrl = browserStreamUrl(output.video_url);
  if (playbackUrl === null || output.render_status !== 'rendered') return null;
  const posterUrl = output.cover_url ? browserStreamUrl(output.cover_url) ?? undefined : undefined;
  let review: PlaybackReview = PLAYBACK_REVIEW.ready;
  if (output.stale) review = PLAYBACK_REVIEW.stale;
  else if (output.review_required) review = PLAYBACK_REVIEW.pending;
  return {
    id: `stream:${job.id}:${output.variant}:${output.artifact_revision}:${output.clip_id}`,
    source: PLAYBACK_SOURCE.stream,
    jobId: job.id,
    variant: output.variant,
    artifactName: output.artifact_name,
    revision: output.artifact_revision,
    title: output.title ?? job.title ?? output.clip_id,
    format: output.aspect_ratio,
    durationSeconds: output.duration_seconds,
    posterUrl,
    playbackUrl,
    review,
    warnings: output.warnings ?? [],
  };
}
