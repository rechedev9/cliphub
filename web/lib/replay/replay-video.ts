import { replayGifFileName } from './replay-gif.ts';
import type { GifSpan } from './replay-gif.ts';

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

/** The GIF's name with the video's extension. */
export function replayVideoFileName(options: Parameters<typeof replayGifFileName>[0]): string {
  return replayGifFileName(options).replace(/\.gif$/, '.mp4');
}
