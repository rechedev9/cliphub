import type { StreamClipRange, StreamPlay } from '../api/streams.ts';
import type { TimelineWindow } from './timeline.ts';

const ZOOM_MARGIN_SECONDS = 45;

export const MIN_MOMENT_SECONDS = 2;

export type MomentEdge = 'start' | 'end';

export function momentZoomWindow(clip: StreamClipRange, sourceDuration: number): TimelineWindow {
  return {
    start: Math.max(0, clip.start_seconds - ZOOM_MARGIN_SECONDS),
    end: Math.min(sourceDuration, clip.end_seconds + ZOOM_MARGIN_SECONDS),
  };
}

type MomentLimits = { min: number; max: number };

function momentLimits(clips: readonly StreamClipRange[], clip: StreamClipRange, sourceDuration: number): MomentLimits {
  let min = 0;
  let max = sourceDuration;
  for (const other of clips) {
    if (other.id === clip.id) continue;
    if (other.end_seconds <= clip.start_seconds) min = Math.max(min, other.end_seconds);
    if (other.start_seconds >= clip.end_seconds) max = Math.min(max, other.start_seconds);
  }
  return { min, max };
}

function withRange(clips: readonly StreamClipRange[], clipId: string, range: { start: number; end: number }): StreamClipRange[] {
  return clips.map((clip) => (clip.id === clipId ? { ...clip, start_seconds: range.start, end_seconds: range.end } : clip));
}

export function moveMomentEdge({
  clips,
  clipId,
  edge,
  seconds,
  sourceDuration,
}: {
  clips: readonly StreamClipRange[];
  clipId: string;
  edge: MomentEdge;
  seconds: number;
  sourceDuration: number;
}): StreamClipRange[] {
  const clip = clips.find((c) => c.id === clipId);
  if (!clip) return [...clips];
  const { min, max } = momentLimits(clips, clip, sourceDuration);
  if (edge === 'start') {
    const start = Math.max(min, Math.min(seconds, clip.end_seconds - MIN_MOMENT_SECONDS));
    return withRange(clips, clipId, { start, end: clip.end_seconds });
  }
  const end = Math.min(max, Math.max(seconds, clip.start_seconds + MIN_MOMENT_SECONDS));
  return withRange(clips, clipId, { start: clip.start_seconds, end });
}

type MomentRange = { start_seconds: number; end_seconds: number };

function overlapSeconds(clip: StreamClipRange, play: StreamPlay): number {
  return Math.min(clip.end_seconds, play.end_seconds) - Math.max(clip.start_seconds, play.start_seconds);
}

export function suggestedCut({
  clips,
  clipId,
  plays,
  sourceDuration,
}: {
  clips: readonly StreamClipRange[];
  clipId: string;
  plays: readonly StreamPlay[];
  sourceDuration: number;
}): MomentRange | null {
  const clip = clips.find((c) => c.id === clipId);
  if (!clip) return null;
  let best: StreamPlay | null = null;
  for (const play of plays) {
    const overlap = overlapSeconds(clip, play);
    if (overlap > 0 && (best === null || overlap > overlapSeconds(clip, best))) best = play;
  }
  if (best === null) return null;
  const { min, max } = momentLimits(clips, clip, sourceDuration);
  const cut = { start_seconds: Math.max(min, best.start_seconds), end_seconds: Math.min(max, best.end_seconds) };
  if (cut.start_seconds === clip.start_seconds && cut.end_seconds === clip.end_seconds) return null;
  return cut;
}

export function withMomentRange({
  clips,
  clipId,
  range,
}: {
  clips: readonly StreamClipRange[];
  clipId: string;
  range: MomentRange;
}): StreamClipRange[] | null {
  if (!clips.some((c) => c.id === clipId)) return null;
  const overlaps = clips.some(
    (other) => other.id !== clipId && other.start_seconds < range.end_seconds && range.start_seconds < other.end_seconds,
  );
  if (overlaps) return null;
  return withRange(clips, clipId, { start: range.start_seconds, end: range.end_seconds });
}

export function nudgeMomentEdge({
  clips,
  clipId,
  edge,
  delta,
  sourceDuration,
}: {
  clips: readonly StreamClipRange[];
  clipId: string;
  edge: MomentEdge;
  delta: number;
  sourceDuration: number;
}): StreamClipRange[] {
  const clip = clips.find((c) => c.id === clipId);
  if (!clip) return [...clips];
  const current = edge === 'start' ? clip.start_seconds : clip.end_seconds;
  return moveMomentEdge({ clips, clipId, edge, seconds: current + delta, sourceDuration });
}
