import type { StreamClipRange, StreamPlay, StreamSummary } from '../api/streams.ts';

/** Best plays first: the highest score, then the earliest. */
export function rankPlays(plays: readonly StreamPlay[]): StreamPlay[] {
  return [...plays].sort((a, b) => b.score - a.score || a.start_seconds - b.start_seconds);
}

/** The second a play is about: its first kill, else its first voice peak. */
export function playMoment(play: StreamPlay): number {
  return play.kill_seconds[0] ?? play.voice_seconds?.[0] ?? play.start_seconds;
}

/** Whether a play is a voice peak with no kill. */
export function isVoiceOnly(play: StreamPlay): boolean {
  return play.kind === 'voice_peak';
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

/**
 * The plays a summary covers, or null when it is unknown. A summary saved
 * before play_ids existed covered every play of its scan.
 */
export function summaryPlayIds(summary: StreamSummary, plays: readonly StreamPlay[]): string[] | null {
  if (summary.play_ids) return summary.play_ids;
  if (summary.status === 'done' && summary.play_count === plays.length) return plays.map((play) => play.id);
  return null;
}

/** Whether both lists hold the same ids, in any order. */
export function sameIds(a: readonly string[], b: readonly string[]): boolean {
  const set = new Set(a);
  return set.size === new Set(b).size && b.every((id) => set.has(id));
}
