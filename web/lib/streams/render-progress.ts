import type { StreamRenderProgress } from '../api/streams.ts';

export type StreamRenderProgressView = {
  /** Whole percent, 0 to 100. */
  percent: number;
  /** 1-based clip FFmpeg is encoding now, out of `clips`. */
  clip: number;
  clips: number;
  /** "Short 2 de 4" */
  clipLabel: string;
};

/** What the export stage and the activity pill show for a running render. */
export function streamRenderProgressView(progress: StreamRenderProgress | undefined): StreamRenderProgressView | null {
  if (progress === undefined || !Number.isFinite(progress.percent) || !(progress.clips >= 1)) return null;
  const clips = Math.floor(progress.clips);
  const clip = Math.min(clips, Math.max(1, Math.floor(progress.clip)));
  return {
    percent: Math.min(100, Math.max(0, Math.round(progress.percent))),
    clip,
    clips,
    clipLabel: `Short ${clip} de ${clips}`,
  };
}
