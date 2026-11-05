import type { StreamClipRange, StreamPlay } from '../api/streams.ts';

/** Best plays first: the highest score, then the earliest. */
export function rankPlays(plays: readonly StreamPlay[]): StreamPlay[] {
  return [...plays].sort((a, b) => b.score - a.score || a.start_seconds - b.start_seconds);
}

/** Whether a play already has a moment that covers part of it. */
export function playIsCovered(clips: readonly StreamClipRange[], play: StreamPlay): boolean {
  return clips.some((clip) => clip.start_seconds < play.end_seconds && play.start_seconds < clip.end_seconds);
}

/** Moments for the plays that do not overlap an existing moment, in source order. */
export function clipsWithPlays(
  clips: readonly StreamClipRange[],
  plays: readonly StreamPlay[],
  newId: () => string,
): StreamClipRange[] {
  const next = [...clips];
  for (const play of plays) {
    if (playIsCovered(next, play)) continue;
    next.push({ id: newId(), start_seconds: play.start_seconds, end_seconds: play.end_seconds, title: play.label });
  }
  return next.sort((a, b) => a.start_seconds - b.start_seconds);
}
