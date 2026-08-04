import type { NormalizedRect, StreamClipRange, StreamTextOverlay, StreamVariant } from './api/streams';

export type FrameSize = { width: number; height: number };

export type CropCoverGeometry = {
  widthPercent: number;
  heightPercent: number;
  leftPercent: number;
  topPercent: number;
};

export type MontagePlaybackCursor = {
  clipIndex: number;
  sourceSeconds: number;
  playbackRate: number;
};

function playableClip(clip: StreamClipRange): boolean {
  return Number.isFinite(clip.start_seconds) &&
    Number.isFinite(clip.end_seconds) &&
    clip.start_seconds >= 0 &&
    clip.end_seconds > clip.start_seconds;
}

function clipPlaybackRate(clip: StreamClipRange): number {
  const speed = clip.edit?.speed ?? 1;
  return Number.isFinite(speed) && speed > 0 ? speed : 1;
}

/** Starts montage playback at the selected clip, never in an excluded source gap. */
export function startMontagePlayback(
  clips: StreamClipRange[],
  sourceSeconds: number,
): MontagePlaybackCursor | null {
  const firstIndex = clips.findIndex(playableClip);
  if (firstIndex < 0) return null;
  const selectedIndex = clips.findIndex((clip) =>
    playableClip(clip) && sourceSeconds >= clip.start_seconds && sourceSeconds < clip.end_seconds,
  );
  const clipIndex = selectedIndex >= 0 ? selectedIndex : firstIndex;
  const clip = clips[clipIndex];
  return {
    clipIndex,
    sourceSeconds: selectedIndex >= 0 ? sourceSeconds : clip.start_seconds,
    playbackRate: clipPlaybackRate(clip),
  };
}

/** Advances within one clip or jumps to the next edited range; null means montage end. */
export function advanceMontagePlayback(
  clips: StreamClipRange[],
  clipIndex: number,
  sourceSeconds: number,
): MontagePlaybackCursor | null {
  const clip = clips[clipIndex];
  if (clip && playableClip(clip) && sourceSeconds < clip.end_seconds) {
    return { clipIndex, sourceSeconds, playbackRate: clipPlaybackRate(clip) };
  }
  for (let index = clipIndex + 1; index < clips.length; index++) {
    const next = clips[index];
    if (!playableClip(next)) continue;
    return {
      clipIndex: index,
      sourceSeconds: next.start_seconds,
      playbackRate: clipPlaybackRate(next),
    };
  }
  return null;
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

export function defaultStreamerBannerPosition(variant: StreamVariant): number {
  return STREAMER_BANNER_DEFAULTS[variant];
}

export function resolveStreamerBannerPosition(variant: StreamVariant, position?: number): number {
  return position === undefined
    ? defaultStreamerBannerPosition(variant)
    : clampStreamerBannerPosition(position);
}

/**
 * Mirrors FFmpeg's crop -> scale(force_original_aspect_ratio=increase) ->
 * centered crop chain for one output band.
 */
export function calculateCropCoverGeometry(
  rect: NormalizedRect,
  source: FrameSize,
  output: FrameSize,
): CropCoverGeometry | null {
  if (
    source.width <= 0 ||
    source.height <= 0 ||
    output.width <= 0 ||
    output.height <= 0 ||
    rect.width <= 0 ||
    rect.height <= 0
  ) {
    return null;
  }

  const cropWidth = source.width * rect.width;
  const cropHeight = source.height * rect.height;
  const scale = Math.max(output.width / cropWidth, output.height / cropHeight);
  const scaledCropWidth = cropWidth * scale;
  const scaledCropHeight = cropHeight * scale;

  return {
    widthPercent: (source.width * scale * 100) / output.width,
    heightPercent: (source.height * scale * 100) / output.height,
    leftPercent: (((output.width - scaledCropWidth) / 2 - source.width * rect.x * scale) * 100) / output.width,
    topPercent: (((output.height - scaledCropHeight) / 2 - source.height * rect.y * scale) * 100) / output.height,
  };
}

/** Selects the same stable, representative frame for every editor video. */
export function representativeFrameTime(duration: number): number {
  if (!Number.isFinite(duration) || duration <= 0) return 0;
  return Math.max(0, Math.min(duration / 2, duration - 0.1));
}

/**
 * The text overlays visible at `frameSeconds`. Overlay windows are relative to
 * the owning clip's start in source seconds (matching the render's drawtext
 * enable windows); missing bounds extend to the clip edges.
 */
export function activeTextOverlays(
  clips: StreamClipRange[],
  frameSeconds: number,
): StreamTextOverlay[] {
  if (!Number.isFinite(frameSeconds)) return [];
  const active: StreamTextOverlay[] = [];
  for (const clip of clips) {
    if (frameSeconds < clip.start_seconds || frameSeconds >= clip.end_seconds) continue;
    const relative = frameSeconds - clip.start_seconds;
    for (const overlay of clip.edit?.text_overlays ?? []) {
      if (overlay.start_seconds !== undefined && relative < overlay.start_seconds) continue;
      if (overlay.end_seconds !== undefined && relative > overlay.end_seconds) continue;
      active.push(overlay);
    }
  }
  return active;
}
