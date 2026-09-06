import type { Play } from '../api/types.ts';

/** A Short should stay at or under one minute. */
export const SHORT_TARGET_SECONDS = 60;

/** Fallback for older plans without source timing. */
const BASE_SECONDS = 6;
const SECONDS_PER_KILL = 3;
type PlayTiming = Pick<Play, 'kills' | 'startSeconds' | 'endSeconds'>;

export function estimatedPlaySeconds(play: PlayTiming): number {
  const { startSeconds, endSeconds } = play;
  if (startSeconds !== undefined && endSeconds !== undefined
    && Number.isFinite(startSeconds) && Number.isFinite(endSeconds) && startSeconds >= 0 && endSeconds > startSeconds) {
    return Math.max(0.1, Math.round((endSeconds - startSeconds) * 10) / 10);
  }
  return BASE_SECONDS + SECONDS_PER_KILL * Math.max(0, play.kills);
}

export function estimatedSelectionSeconds(plays: ReadonlyArray<PlayTiming>): number {
  return plays.reduce((total, play) => total + estimatedPlayTenths(play), 0) / 10;
}

/** Accumulate the displayed precision as integers so an exact minute stays within budget. */
function estimatedPlayTenths(play: PlayTiming): number {
  return Math.round(estimatedPlaySeconds(play) * 10);
}

/** `m:ss`, e.g. 47 → "0:47", 75 → "1:15". Clamped to zero first, so -0 or a tiny negative never prints "-0:00". */
export function formatClock(totalSeconds: number): string {
  const whole = Math.max(0, Math.round(totalSeconds));
  const minutes = Math.floor(whole / 60);
  const seconds = whole % 60;
  return `${minutes}:${String(seconds).padStart(2, '0')}`;
}

/**
 * Greedy best-of: highest kills first (plan order breaks ties), adding a
 * highlight only while the estimated total stays within the target. The
 * top-ranked highlight is always kept, so "Auto" never returns an empty
 * selection just because one long play overruns the target on its own.
 */
export function autoPickBestPlays(plays: readonly Play[], targetSeconds = SHORT_TARGET_SECONDS): Set<string> {
  const ranked = plays
    .map((play, index) => ({ play, index }))
    .sort((a, b) => b.play.kills - a.play.kills || a.index - b.index);
  const picked = new Set<string>();
  let totalTenths = 0;
  for (const { play } of ranked) {
    const tenths = estimatedPlayTenths(play);
    if ((totalTenths + tenths) / 10 > targetSeconds) continue;
    picked.add(play.id);
    totalTenths += tenths;
  }
  if (picked.size === 0 && ranked.length > 0) picked.add(ranked[0].play.id);
  return picked;
}

/** `R7 · R12 · R19` in plan order; rounds repeat when two highlights share one. */
export function roundsSummary(plays: ReadonlyArray<Pick<Play, 'round'>>): string {
  return plays.map((play) => `R${play.round}`).join(' · ');
}

export type SelectionCue = {
  play: Play;
  /** Estimated length of this highlight in the Short. */
  seconds: number;
  /** Estimated second at which it starts, in plan order. */
  startAt: number;
};

/** The Short as a running order: plan order, each cue with its estimated length and start. */
export function selectionTimeline(plays: readonly Play[], selectedIds: ReadonlySet<string>): SelectionCue[] {
  const cues: SelectionCue[] = [];
  let elapsedTenths = 0;
  for (const play of plays) {
    if (!selectedIds.has(play.id)) continue;
    const tenths = estimatedPlayTenths(play);
    cues.push({ play, seconds: tenths / 10, startAt: elapsedTenths / 10 });
    elapsedTenths += tenths;
  }
  return cues;
}
