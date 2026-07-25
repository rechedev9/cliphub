import { SERVICE_UNAVAILABLE_CODE } from '../api/types.ts';
import {
  CAPTION_GENERATION_STATUS,
  KILLFEED_ANALYSIS_STATUS,
  type CaptionGenerationState,
  type KillfeedAnalysisState,
  type NormalizedRect,
  type StreamCaptionWord,
  type StreamClipEdit,
  type StreamClipRange,
  type StreamEditPlan,
  type StreamTextOverlay,
  type StreamVariant,
} from '../api/streams.ts';
import { DEFAULT_OVERLAY_FONT_SIZE } from '../clip-edit.ts';
import { initialStreamClipEnd } from '../killfeed-plan.ts';

/**
 * Pure helpers behind the Clips de stream editor: source validation, plan
 * construction, the render fingerprint, and the geometry the timeline and the
 * caption reviewer draw from. Everything here is deterministic and free of
 * React, the DOM and `streamsApi`, so `plan.test.ts` can pin the behaviour the
 * screen depends on.
 */

/** Schema the editor writes; mirrors streamclips.EditPlan. */
export const EDIT_PLAN_SCHEMA_VERSION = '1.1';

export const FULL_FRAME: NormalizedRect = { x: 0, y: 0, width: 1, height: 1 };
export const DEFAULT_FACE_CROP: NormalizedRect = { x: 0.62, y: 0.03, width: 0.34, height: 0.3 };
export const DEFAULT_KILLFEED_CROP: NormalizedRect = { x: 0.68, y: 0.04, width: 0.31, height: 0.14 };
export const KILLFEED_MIN_CROP_SIZE = 0.02;

/** Nicks the streamer banner accepts, matching the Go validator. */
export const STREAMER_NICK_RE = /^[A-Za-z0-9_]{0,25}$/;

/** Sentinel for the "no music" row: Radix Select forbids an empty value. */
export const NO_MUSIC_VALUE = '__none__';

/** Upstream code for a killfeed-read blocked by a missing xAI key. */
export const XAI_KEY_MISSING_CODE = 'xai_key_missing';

/** Preset music gains: quiet bed, balanced, or music-forward. */
export const MUSIC_VOLUMES: readonly { value: number; label: string }[] = [
  { value: 0.15, label: 'Bajo' },
  { value: 0.25, label: 'Medio' },
  { value: 0.4, label: 'Alto' },
];

/** The one offline sentence every stream call falls back to. */
export const STREAM_OFFLINE_MESSAGE =
  'El servicio de Clips de stream está offline. Arráncalo y vuelve a intentarlo.';

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/** True when an API error means the local analysis service is unreachable. */
export function isServiceUnavailable(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === SERVICE_UNAVAILABLE_CODE;
}

/** True when an API error is the xAI key gate rather than a real failure. */
export function isMissingXaiKey(err: unknown): boolean {
  return (err as { code?: string } | null)?.code === XAI_KEY_MISSING_CODE;
}

/** Localized message for a failed API call, preferring the offline hint. */
export function errorMessage(err: unknown, fallback: string): string {
  if (isServiceUnavailable(err)) {
    return STREAM_OFFLINE_MESSAGE;
  }
  if (err instanceof Error) {
    return err.message;
  }
  return fallback;
}

/**
 * Extensions of URLs that are a direct link to a non-video asset (an image
 * pasted from a clipboard uploader like ShareX, a document, an archive, an
 * audio-only file). yt-dlp cannot turn these into an MP4, so we reject them
 * instantly with a localized message instead of round-tripping to a doomed
 * acquire job. The server guards the same set (vodfetch.ClassifySource) for
 * direct API callers; this is the fast, in-language UX path.
 */
const NON_VIDEO_EXT_RE =
  /\.(png|jpe?g|gif|webp|bmp|svg|ico|tiff?|heic|avif|pdf|txt|md|csv|json|xml|html?|zip|rar|7z|gz|tar|mp3|wav|flac|ogg|m4a|docx?|xlsx?)$/i;

export function isStreamURLValidationError(message: string | null): message is string {
  return message?.startsWith('Pega una URL') === true || message?.startsWith('Esa URL') === true;
}

/**
 * The extension (without the dot, lowercased) if `raw` clearly points to a
 * non-video file, else null. Unparseable input is left for the server.
 */
export function nonVideoExtension(raw: string): string | null {
  try {
    const match = new URL(raw).pathname.match(NON_VIDEO_EXT_RE);
    return match ? match[1].toLowerCase() : null;
  } catch {
    return null;
  }
}

export function streamSourceLabel(sourceUrl?: string): string | null {
  if (!sourceUrl) return null;
  try {
    const url = new URL(sourceUrl);
    if (url.hostname.endsWith('twitch.tv')) {
      const parts = url.pathname.split('/').filter(Boolean);
      const channel = parts.length === 3 && parts[1] === 'clip' ? parts[0] : null;
      return channel ? `Twitch · ${channel}` : 'Twitch';
    }
    if (url.hostname.endsWith('youtube.com') || url.hostname === 'youtu.be') return 'YouTube';
    return url.hostname;
  } catch {
    return null;
  }
}

let clipSeq = 0;

export function nextClipId(): string {
  clipSeq += 1;
  return `clip-${Date.now()}-${clipSeq}`;
}

/** A fresh range covering the head of the source, as the editor first offers it. */
export function blankClip(durationSeconds: number): StreamClipRange {
  return { id: nextClipId(), start_seconds: 0, end_seconds: initialStreamClipEnd(durationSeconds), title: '' };
}

export function blankPlan(
  durationSeconds = 0,
  variant: StreamVariant = 'streamer-vertical-stack-40-60',
): StreamEditPlan {
  return {
    schema_version: EDIT_PLAN_SCHEMA_VERSION,
    variant,
    face_crop: DEFAULT_FACE_CROP,
    face_crop_reviewed: false,
    gameplay_crop: FULL_FRAME,
    clips: [blankClip(durationSeconds)],
    captions: { enabled: false, language: 'es' },
  };
}

/** True once every clip range in the plan is well-formed (end strictly after start). */
export function clipsAreValid(clips: StreamClipRange[]): boolean {
  return (
    clips.length > 0 &&
    clips.every(
      (c) =>
        Number.isFinite(c.start_seconds) && Number.isFinite(c.end_seconds) && c.end_seconds > c.start_seconds,
    )
  );
}

export function formatStreamTimestamp(seconds: number): string {
  const safeSeconds = Number.isFinite(seconds) ? Math.max(0, seconds) : 0;
  const minutes = Math.floor(safeSeconds / 60);
  const remainder = safeSeconds - minutes * 60;
  return `${minutes}:${remainder.toFixed(2).padStart(5, '0')}`;
}

/**
 * Canonical fingerprint of everything a render consumes from the plan, so the
 * UI can tell whether the shown Shorts still match the current edits. Fields
 * are listed explicitly (not JSON.stringify of the object) so key order and
 * volatile fields like updated_at can never cause a false mismatch.
 */
export function planFingerprint(plan: StreamEditPlan): string {
  const rect = (r?: NormalizedRect) => (r ? [r.x, r.y, r.width, r.height] : null);
  const overlay = (o: StreamTextOverlay) => [
    o.text,
    o.position_y,
    o.start_seconds ?? null,
    o.end_seconds ?? null,
    o.font_size ?? DEFAULT_OVERLAY_FONT_SIZE,
  ];
  // Defaults collapse an absent edit and an all-defaults edit to the same key.
  const edit = (e?: StreamClipEdit) => [
    e?.speed ?? 1,
    e?.source_volume ?? 1,
    e?.fade_in_seconds ?? 0,
    e?.fade_out_seconds ?? 0,
    (e?.text_overlays ?? []).map(overlay),
  ];
  return JSON.stringify({
    variant: plan.variant,
    face: rect(plan.face_crop),
    faceReviewed: plan.face_crop_reviewed ?? false,
    killfeed: rect(plan.killfeed_crop),
    killfeedAnalysis: [
      plan.killfeed_analysis?.generation_id ?? '',
      plan.killfeed_analysis?.fingerprint ?? '',
    ],
    game: rect(plan.gameplay_crop),
    clips: plan.clips.map((c) => [
      c.id,
      c.start_seconds,
      c.end_seconds,
      c.title ?? '',
      c.killfeed_seconds ?? [],
      c.killfeed_kills ?? [],
      (c.caption_words ?? []).map((word) => [word.word, word.start_seconds, word.end_seconds]),
      c.caption_reviewed ?? false,
      edit(c.edit),
    ]),
    streamerNick: plan.streamer_banner?.nick?.trim() ?? '',
    streamerPosition: plan.streamer_banner?.position_y ?? null,
    streamerSlide: plan.streamer_banner?.slide_enabled ?? false,
    captions: [plan.captions?.enabled ?? false, 'es'],
    music: [plan.music?.key ?? '', plan.music?.volume ?? 0],
    grade: plan.effects?.grade ?? false,
  });
}

export function captionGenerationIsPending(state: CaptionGenerationState | null): boolean {
  return (
    state?.status === CAPTION_GENERATION_STATUS.queued ||
    state?.status === CAPTION_GENERATION_STATUS.generating
  );
}

export function killfeedAnalysisIsPending(state: KillfeedAnalysisState | null): boolean {
  return (
    state?.status === KILLFEED_ANALYSIS_STATUS.queued ||
    state?.status === KILLFEED_ANALYSIS_STATUS.analyzing
  );
}

export function captionDraftsFromState(state: CaptionGenerationState): Record<string, StreamCaptionWord[]> {
  return Object.fromEntries(
    (state.clips ?? []).map((clip) => [
      clip.clip_id,
      (clip.candidate_words ?? []).map((word) => ({ ...word })),
    ]),
  );
}

/** Total number of aligned events across every analysed clip. */
export function detectedKillfeedEventCount(state: KillfeedAnalysisState | null): number {
  return (state?.clips ?? []).reduce((total, clip) => total + clip.events.length, 0);
}

/**
 * Drops default-valued fields so an untouched edit keeps the plan (and the
 * render fingerprint) identical to a plan without an `edit` object at all.
 */
export function pruneClipEdit(edit: StreamClipEdit): StreamClipEdit | undefined {
  const next: StreamClipEdit = {};
  if (edit.speed !== undefined && edit.speed !== 1) next.speed = edit.speed;
  if (edit.source_volume !== undefined && edit.source_volume !== 1) next.source_volume = edit.source_volume;
  if (edit.fade_in_seconds) next.fade_in_seconds = edit.fade_in_seconds;
  if (edit.fade_out_seconds) next.fade_out_seconds = edit.fade_out_seconds;
  if (edit.text_overlays && edit.text_overlays.length > 0) next.text_overlays = edit.text_overlays;
  return Object.keys(next).length > 0 ? next : undefined;
}

/** Source seconds the clip spans, or 0 for a malformed range. */
export function clipSourceDuration(clip: StreamClipRange): number {
  const span = clip.end_seconds - clip.start_seconds;
  return Number.isFinite(span) && span > 0 ? span : 0;
}

/** Seconds the clip occupies in the finished Short, after the speed change. */
export function clipOutputDuration(clip: StreamClipRange): number {
  const speed = clip.edit?.speed ?? 1;
  return speed > 0 ? clipSourceDuration(clip) / speed : 0;
}

export type ClipTimelineGeometry = {
  /** Where the clip starts on the source timeline, as a 0..100 percentage. */
  startPercent: number;
  /** How much of the source timeline the clip covers, as a 0..100 percentage. */
  widthPercent: number;
  /** Fade-in width as a percentage OF THE CLIP, 0 when there is no fade. */
  fadeInPercent: number;
  fadeOutPercent: number;
};

/**
 * The clip band drawn over the source timeline. Returns null when the probe has
 * not reported a duration yet — the editor must not invent a scale for a video
 * whose length it does not know.
 */
export function clipTimelineGeometry(
  clip: StreamClipRange,
  sourceDuration: number,
): ClipTimelineGeometry | null {
  if (!Number.isFinite(sourceDuration) || sourceDuration <= 0) return null;
  const span = clipSourceDuration(clip);
  if (span <= 0) return null;
  const start = Math.min(Math.max(clip.start_seconds, 0), sourceDuration);
  const width = Math.min(span, sourceDuration - start);
  const outputSeconds = clipOutputDuration(clip);
  const fadeShare = (seconds: number | undefined): number => {
    if (!seconds || seconds <= 0 || outputSeconds <= 0) return 0;
    return Math.min(100, (seconds / outputSeconds) * 100);
  };
  return {
    startPercent: (start / sourceDuration) * 100,
    widthPercent: Math.max(0, (width / sourceDuration) * 100),
    fadeInPercent: fadeShare(clip.edit?.fade_in_seconds),
    fadeOutPercent: fadeShare(clip.edit?.fade_out_seconds),
  };
}

export type OverlayMarkerGeometry = { startPercent: number; widthPercent: number };

/**
 * Where a text overlay sits inside its clip, as percentages of the clip band.
 * An absent bound extends to the corresponding clip edge, exactly as the render
 * treats it.
 */
export function overlayMarkerGeometry(
  overlay: StreamTextOverlay,
  clipDuration: number,
): OverlayMarkerGeometry | null {
  if (!Number.isFinite(clipDuration) || clipDuration <= 0) return null;
  const rawStart = overlay.start_seconds ?? 0;
  const rawEnd = overlay.end_seconds ?? clipDuration;
  const start = Math.min(Math.max(rawStart, 0), clipDuration);
  const end = Math.min(Math.max(rawEnd, start), clipDuration);
  return {
    startPercent: (start / clipDuration) * 100,
    widthPercent: Math.max(1, ((end - start) / clipDuration) * 100),
  };
}

/** Pause (in seconds) that starts a new caption line in the review card. */
export const CAPTION_SEGMENT_GAP_SECONDS = 0.6;

export type CaptionWordEntry = {
  /** Index into the flat word list the reviewer edits. */
  index: number;
  word: StreamCaptionWord;
};

export type CaptionSegment = {
  entries: CaptionWordEntry[];
  startSeconds: number;
  endSeconds: number;
  /** The line as it will be read on screen. */
  text: string;
};

/**
 * Groups a flat word list into readable lines so a 90-word clip stops rendering
 * as 90 anonymous three-field rows. Purely presentational: the flat indices are
 * carried through, so every edit still targets the same word in the same order,
 * and the grouping never changes what is written to the plan.
 *
 * A new line starts on a pause longer than `gapSeconds` or after sentence-final
 * punctuation, which is how the transcript reads out loud.
 */
export function groupCaptionWords(
  words: StreamCaptionWord[],
  gapSeconds = CAPTION_SEGMENT_GAP_SECONDS,
): CaptionSegment[] {
  const segments: CaptionSegment[] = [];
  let current: CaptionWordEntry[] = [];

  const flush = (): void => {
    if (current.length === 0) return;
    const first = current[0].word;
    const last = current[current.length - 1].word;
    segments.push({
      entries: current,
      startSeconds: first.start_seconds,
      endSeconds: last.end_seconds,
      text: current
        .map((entry) => entry.word.word.trim())
        .filter((word) => word !== '')
        .join(' '),
    });
    current = [];
  };

  for (const [index, word] of words.entries()) {
    const previous = current[current.length - 1]?.word;
    const gap = previous ? word.start_seconds - previous.end_seconds : 0;
    const endsSentence = previous ? /[.!?…]$/.test(previous.word.trim()) : false;
    if (previous && (gap > gapSeconds || endsSentence)) flush();
    current.push({ index, word });
  }
  flush();

  return segments;
}
