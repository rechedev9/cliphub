import type { StreamClipRange } from '../api/streams.ts';
import { MIN_MOMENT_SECONDS } from './moment-edges.ts';

const MARK_BEFORE_SECONDS = 5;
const MARK_AFTER_SECONDS = 10;
const SHORT_SOURCE_MAX_SECONDS = 180;

type MomentRange = { start_seconds: number; end_seconds: number };

export function fullVideoMoment({
  clips,
  sourceDuration,
}: {
  clips: readonly StreamClipRange[];
  sourceDuration: number;
}): MomentRange | null {
  if (clips.length > 0 || !(sourceDuration > 0) || sourceDuration > SHORT_SOURCE_MAX_SECONDS) return null;
  return { start_seconds: 0, end_seconds: sourceDuration };
}

export type MarkOutcome =
  | { kind: 'created'; range: MomentRange }
  | { kind: 'inside'; clip: StreamClipRange }
  | { kind: 'no-room' };

export function markMomentAt({
  clips,
  seconds,
  sourceDuration,
}: {
  clips: readonly StreamClipRange[];
  seconds: number;
  sourceDuration: number;
}): MarkOutcome {
  const inside = clips.find((clip) => seconds >= clip.start_seconds && seconds < clip.end_seconds);
  if (inside) return { kind: 'inside', clip: inside };
  let min = 0;
  let max = sourceDuration;
  for (const clip of clips) {
    if (clip.end_seconds <= seconds) min = Math.max(min, clip.end_seconds);
    if (clip.start_seconds > seconds) max = Math.min(max, clip.start_seconds);
  }
  const start = Math.max(min, seconds - MARK_BEFORE_SECONDS);
  const end = Math.min(max, seconds + MARK_AFTER_SECONDS);
  if (end - start < MIN_MOMENT_SECONDS) return { kind: 'no-room' };
  return { kind: 'created', range: { start_seconds: start, end_seconds: end } };
}
