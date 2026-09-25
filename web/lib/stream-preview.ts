import type { StreamClipRange, StreamVariant } from './api/streams';

export type FrameSize = { width: number; height: number };

function playableClip(clip: StreamClipRange): boolean {
  return Number.isFinite(clip.start_seconds) &&
    Number.isFinite(clip.end_seconds) &&
    clip.start_seconds >= 0 &&
    clip.end_seconds > clip.start_seconds;
}

export const STREAMER_BANNER_MIN_POSITION = 0.025;
export const STREAMER_BANNER_MAX_POSITION = 0.975;

/** Shared vertical bounds for streamer and KeyDrop banners. */
export const KEYDROP_BANNER_MIN_POSITION = STREAMER_BANNER_MIN_POSITION;
export const KEYDROP_BANNER_MAX_POSITION = STREAMER_BANNER_MAX_POSITION;
export const KEYDROP_BANNER_DEFAULT_POSITION = 0.86;

export function clampKeyDropBannerPosition(position: number): number {
  return Math.min(KEYDROP_BANNER_MAX_POSITION, Math.max(KEYDROP_BANNER_MIN_POSITION, position));
}

export function resolveKeyDropBannerPosition(position?: number): number {
  return position === undefined
    ? KEYDROP_BANNER_DEFAULT_POSITION
    : clampKeyDropBannerPosition(position);
}

const STREAMER_BANNER_DEFAULTS: Record<StreamVariant, number> = {
  'streamer-vertical-stack-40-60': 0.374,
  'streamer-vertical-stack': 520 / 1920,
  'streamer-fullframe-nocam': 0.2,
};

export function clampStreamerBannerPosition(position: number): number {
  return Math.min(STREAMER_BANNER_MAX_POSITION, Math.max(STREAMER_BANNER_MIN_POSITION, position));
}

export function resolveStreamerBannerPosition(variant: StreamVariant, position?: number): number {
  return position === undefined
    ? STREAMER_BANNER_DEFAULTS[variant]
    : clampStreamerBannerPosition(position);
}

/**
 * Source time where the KeyDrop plate is on-screen. Keeps the playhead when it
 * already sits inside a clip's plate window; otherwise jumps to the plate's
 * start on the active or first playable clip so code edits are visible.
 * (The default editor frame is the source midpoint, which almost never falls
 * inside the product default 0–4s callout.)
 */
export function keyDropPreviewSourceSeconds(
  clips: readonly StreamClipRange[],
  sourceSeconds: number,
  startSeconds: number,
  endSeconds: number,
): number {
  const start = Number.isFinite(startSeconds) && startSeconds >= 0 ? startSeconds : 0;
  const end =
    Number.isFinite(endSeconds) && endSeconds > start ? endSeconds : start + 4;

  const seekInClip = (clip: StreamClipRange): number => {
    const clipLen = clip.end_seconds - clip.start_seconds;
    const local = Math.min(start, Math.max(0, clipLen - 0.05));
    return clip.start_seconds + local;
  };

  const active = clips.find(
    (clip) =>
      playableClip(clip) &&
      sourceSeconds >= clip.start_seconds &&
      sourceSeconds < clip.end_seconds,
  );
  if (active) {
    const local = sourceSeconds - active.start_seconds;
    if (local >= start && local < end) return sourceSeconds;
    return seekInClip(active);
  }
  const first = clips.find(playableClip);
  return first ? seekInClip(first) : sourceSeconds;
}
