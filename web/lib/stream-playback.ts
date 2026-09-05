import type { StreamClipRange } from './api/streams.ts';
import type { PlaybackRange } from './playback-session.ts';

export const STREAM_PLAYBACK_MODE = { source: 'source', selected: 'selected', sequence: 'sequence' } as const;
export type StreamPlaybackMode = (typeof STREAM_PLAYBACK_MODE)[keyof typeof STREAM_PLAYBACK_MODE];

export function streamClipRange(clip: StreamClipRange | undefined): PlaybackRange | null {
  if (!clip || !Number.isFinite(clip.start_seconds) || !Number.isFinite(clip.end_seconds)
    || clip.start_seconds < 0 || clip.end_seconds <= clip.start_seconds) return null;
  const rate = clip.edit?.speed ?? 1;
  return { start: clip.start_seconds, end: clip.end_seconds, rate: Number.isFinite(rate) && rate > 0 ? rate : 1 };
}

export function streamPlaybackIndex(clips: readonly StreamClipRange[], selectedId: string | null): number {
  const selected = clips.findIndex((clip) => clip.id === selectedId && streamClipRange(clip) !== null);
  return selected >= 0 ? selected : clips.findIndex((clip) => streamClipRange(clip) !== null);
}

export function nextStreamPlaybackIndex(clips: readonly StreamClipRange[], current: number, mode: StreamPlaybackMode, loop: boolean): number {
  if (mode === STREAM_PLAYBACK_MODE.source) return -1;
  if (mode === STREAM_PLAYBACK_MODE.sequence) {
    const next = clips.findIndex((clip, index) => index > current && streamClipRange(clip) !== null);
    if (next >= 0) return next;
    return loop ? streamPlaybackIndex(clips, null) : -1;
  }
  return loop && streamClipRange(clips[current]) !== null ? current : -1;
}

export function streamPreviewFade(clip: StreamClipRange | undefined, sourceSeconds: number): number {
  const range = streamClipRange(clip);
  if (!clip || !range) return 1;
  const time = (sourceSeconds - range.start) / range.rate;
  const duration = (range.end - range.start) / range.rate;
  const fadeIn = clip.edit?.fade_in_seconds ?? 0;
  const fadeOut = clip.edit?.fade_out_seconds ?? 0;
  return streamFadeEnvelope(time, duration, fadeIn, fadeOut);
}

/** Mirrors consecutive FFmpeg fade/afade filters, including overlap multiplication. */
export function streamFadeEnvelope(
  outputSeconds: number,
  outputDuration: number,
  fadeInSeconds: number,
  fadeOutSeconds: number,
): number {
  const time = Number.isFinite(outputSeconds) ? outputSeconds : 0;
  const duration = Number.isFinite(outputDuration) ? Math.max(0, outputDuration) : 0;
  const fadeIn = Number.isFinite(fadeInSeconds) && fadeInSeconds > 0 ? fadeInSeconds : 0;
  const fadeOut = Number.isFinite(fadeOutSeconds) && fadeOutSeconds > 0 ? fadeOutSeconds : 0;
  const incoming = fadeIn > 0 ? clamp(time / fadeIn, 0, 1) : 1;
  const outgoing = fadeOut > 0 ? clamp((duration - time) / fadeOut, 0, 1) : 1;
  return incoming * outgoing;
}

export function streamBannerSlide(sourceSeconds: number, duration: number): number {
  const phase = Math.min(0.35, duration / 2);
  if (phase <= 0 || sourceSeconds < 0 || sourceSeconds > duration) return -100;
  if (sourceSeconds < phase) return -100 * (1 - sourceSeconds / phase);
  if (sourceSeconds > duration - phase) return -100 * (sourceSeconds - duration + phase) / phase;
  return 0;
}

export type StreamAffiliateWindow = { start: number; end: number };

/** Mirrors keydropbanner.resolveVisibleWindow for preview visibility and motion. */
export function streamAffiliateWindow(
  startSeconds: number,
  endSeconds: number,
  durationSeconds: number,
): StreamAffiliateWindow {
  if (!Number.isFinite(durationSeconds) || durationSeconds <= 0) return { start: 0, end: 0 };
  let start = Number.isFinite(startSeconds) && startSeconds >= 0 ? startSeconds : 0;
  let end = Number.isFinite(endSeconds) && endSeconds > 0 && endSeconds <= durationSeconds
    ? endSeconds
    : durationSeconds;
  if (start >= durationSeconds) start = 0;
  if (end <= start) end = durationSeconds;
  return { start, end };
}

/**
 * Affiliate plate translation as a percentage of its own width. The Go filter
 * positions the plate with an absolute x expression around a centered hold,
 * which is intentionally different from the full-width streamer banner.
 */
export function streamAffiliateSlide(
  sourceSeconds: number,
  startSeconds: number,
  endSeconds: number,
  durationSeconds: number,
  overlayWidthFraction = 0.55,
): number {
  const window = streamAffiliateWindow(startSeconds, endSeconds, durationSeconds);
  const width = Number.isFinite(overlayWidthFraction) && overlayWidthFraction > 0
    ? overlayWidthFraction
    : 0.55;
  const time = Number.isFinite(sourceSeconds) ? sourceSeconds : window.start;
  const phase = Math.min(0.35, (window.end - window.start) / 2);
  if (phase <= 0) return 0;
  const centerInOverlayWidths = (1 - width) / (2 * width);
  if (time < window.start + phase) {
    const progress = clamp((time - window.start) / phase, 0, 1);
    return 100 * (-1 - centerInOverlayWidths * (1 - progress));
  }
  if (time < window.end - phase) return 0;
  const progress = clamp((time - (window.end - phase)) / phase, 0, 1);
  return progress === 0 ? 0 : -100 * progress;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
