import { replayGifFileName } from './replay-gif.ts';
import type { GifSpan } from './replay-gif.ts';
import { audioSecondsAt, demoMsOfTick } from './voice-sync.ts';

/** The MP4 of the Replay's tramo: its frame times, size and file name. components/replay/video-export.ts draws it. */

export const VIDEO_FPS = 30;
/** The orchestrator's cap on one video (`replayvideo.MaxSeconds`). */
export const VIDEO_MAX_SECONDS = 120;
export const VIDEO_MAX_SIDE = 1920;
const VIDEO_MIN_SIDE = 64;
export const VIDEO_JPEG_QUALITY = 0.92;

/** How long the video plays a span at `speed`. */
export function videoSeconds(span: GifSpan, speed: number): number {
  return (span.to - span.from) / (speed > 0 ? speed : 1);
}

/** The transport's clock at each frame of a span played at `speed`. */
export function videoFrameTimes(span: GifSpan, speed: number, fps: number = VIDEO_FPS): number[] {
  const rate = speed > 0 ? speed : 1;
  const count = Math.max(1, Math.round(videoSeconds(span, rate) * fps));
  return Array.from({ length: count }, (_, index) => span.from + (index / fps) * rate);
}

function even(value: number): number {
  return Math.max(VIDEO_MIN_SIDE, 2 * Math.floor(value / 2));
}

/**
 * The video's size from the map's canvas as the Replay draws it, its longer side at most
 * `VIDEO_MAX_SIDE` and both sides even, as H.264 asks; `scale` maps the map's box into it.
 */
export function videoSize(canvas: { width: number; height: number }, box: { width: number }): { width: number; height: number; scale: number } {
  const shrink = Math.min(1, VIDEO_MAX_SIDE / Math.max(1, canvas.width, canvas.height));
  const width = even(canvas.width * shrink);
  const height = even(canvas.height * shrink);
  return { width, height, scale: width / Math.max(1, box.width) };
}

/** The speeds the orchestrator encodes (`replayvideo.Speeds`): every speed of the Replay's transport. */
export const VIDEO_SPEEDS: readonly number[] = [0.25, 0.5, 1, 2, 4];

/** Why a tramo cannot become an MP4 at this speed, checked before a single frame is drawn; null when it can. */
export function videoBlocked(at: { span: GifSpan; speed: number }): string | null {
  if (!VIDEO_SPEEDS.includes(at.speed)) return 'El MP4 se exporta a 0,25×, 0,5×, 1×, 2× o 4×: cambia la velocidad.';
  if (videoSeconds(at.span, at.speed) > VIDEO_MAX_SECONDS) return `El MP4 dura como mucho ${VIDEO_MAX_SECONDS} s: elige un tramo más corto o más velocidad.`;
  return null;
}

/** Where the voice goes quiet: the end of freeze time with "Solo tiempo de compra" on, else the end of the stretch. */
export function voiceEndTick(at: { fromTick: number; toTick: number; freezeOnly: boolean; freezeEndTick: number | null }): number {
  if (!at.freezeOnly || at.freezeEndTick === null) return at.toTick;
  return Math.max(at.fromTick, Math.min(at.toTick, at.freezeEndTick));
}

/** Whether any track recorded part of the demo ticks `fromTick` to `toTick` at this offset. */
export function voiceCovers(at: { fromTick: number; toTick: number; tickrate: number; offsetMs: number; durationsMs: readonly number[] }): boolean {
  if (at.toTick <= at.fromTick) return false;
  const { tickrate, offsetMs } = at;
  return at.durationsMs.some((durationMs) => {
    const sync = { tickrate, offsetMs, durationMs };
    if (audioSecondsAt({ ...sync, tick: at.fromTick }) !== null || audioSecondsAt({ ...sync, tick: at.toTick }) !== null) return true;
    // The stretch can also hold the whole recording, starting before it and ending after it.
    return demoMsOfTick(at.fromTick, tickrate) - offsetMs < 0 && demoMsOfTick(at.toTick, tickrate) - offsetMs > durationMs;
  });
}

/** The GIF's name with the video's extension. */
export function replayVideoFileName(options: Parameters<typeof replayGifFileName>[0]): string {
  return replayGifFileName(options).replace(/\.gif$/, '.mp4');
}
