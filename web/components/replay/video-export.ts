import { VIDEO_JPEG_QUALITY, videoSize } from '@/lib/replay/replay-video';
import { framePainter } from './gif-export';
import type { GifOverlay, MapCapture } from './gif-export';

function jpeg(canvas: HTMLCanvasElement): Promise<Blob> {
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => (blob === null ? reject(new Error('the frame could not be encoded')) : resolve(blob)), 'image/jpeg', VIDEO_JPEG_QUALITY);
  });
}

/**
 * Draws every frame of the MP4 off screen at the map canvas's own size, with the drawings and
 * the HUD's line, as JPEGs for the orchestrator to encode. Throws when `signal` aborts.
 */
export async function renderReplayVideo(options: {
  capture: MapCapture;
  times: readonly number[];
  overlay: GifOverlay;
  onProgress: (done: number) => void;
  signal: AbortSignal;
}): Promise<{ frames: Blob[]; width: number; height: number }> {
  const { capture, times, overlay, onProgress, signal } = options;
  const first = capture.paint(times[0] ?? 0);
  if (first === null) throw new Error('the map could not draw');
  const size = videoSize(first.canvas, capture.box);
  const painter = framePainter({ capture, overlay, size });
  const frames: Blob[] = [];
  for (const [done, seconds] of times.entries()) {
    if (signal.aborted) throw new Error('cancelled');
    painter.paint(seconds);
    frames.push(await jpeg(painter.canvas));
    onProgress(done + 1);
  }
  return { frames, width: size.width, height: size.height };
}
