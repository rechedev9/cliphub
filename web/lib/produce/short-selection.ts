import type { Play } from '../api/types.ts';
import { clashingPlay } from './multi-pov.ts';

/** A Short should stay at or under one minute. */
export const SHORT_TARGET_SECONDS = 60;

/** Fallback for older plans without source timing. */
const BASE_SECONDS = 6;
const SECONDS_PER_KILL = 3;
type PlayTiming = Pick<Play, 'kills' | 'startSeconds' | 'endSeconds' | 'victimReplay'>;
type ReplayOption = { victimReplay?: boolean };

export function estimatedPlaySeconds(play: PlayTiming): number {
  const { startSeconds, endSeconds } = play;
  if (startSeconds !== undefined && endSeconds !== undefined
    && Number.isFinite(startSeconds) && Number.isFinite(endSeconds) && startSeconds >= 0 && endSeconds > startSeconds) {
    return Math.max(0.1, Math.round((endSeconds - startSeconds) * 10) / 10);
  }
  return BASE_SECONDS + SECONDS_PER_KILL * Math.max(0, play.kills);
}

/** A victim replay is about 3.5 s of capture played at half speed. */
export const VICTIM_REPLAY_SECONDS = 7;

export function estimatedSelectionSeconds(plays: ReadonlyArray<PlayTiming>, options: ReplayOption = {}): number {
  return plays.reduce((total, play) => total + estimatedPlayTenths(play, options), 0) / 10;
}

/** Accumulate the displayed precision as integers so an exact minute stays within budget. */
function estimatedPlayTenths(play: PlayTiming, options: ReplayOption): number {
  const replay = options.victimReplay && play.victimReplay !== false ? VICTIM_REPLAY_SECONDS : 0;
  return Math.round((estimatedPlaySeconds(play) + replay) * 10);
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
export function autoPickBestPlays(
  plays: readonly Play[],
  options: ReplayOption & { targetSeconds?: number } = {},
): Set<string> {
  const targetSeconds = options.targetSeconds ?? SHORT_TARGET_SECONDS;
  const ranked = plays
    .map((play, index) => ({ play, index }))
    .sort((a, b) => b.play.kills - a.play.kills || a.index - b.index);
  const picked = new Set<string>();
  const pickedPlays: Play[] = [];
  let totalTenths = 0;
  for (const { play } of ranked) {
    const tenths = estimatedPlayTenths(play, options);
    // A multi-POV capture cannot film two players at once.
    if ((totalTenths + tenths) / 10 > targetSeconds || clashingPlay(play, pickedPlays)) continue;
    picked.add(play.id);
    pickedPlays.push(play);
    totalTenths += tenths;
  }
  if (picked.size === 0 && ranked.length > 0) picked.add(ranked[0].play.id);
  return picked;
}

/** `R7 · R12 ×2 · R19` in plan order; a round with several highlights shows its count. */
export function roundsSummary(plays: ReadonlyArray<Pick<Play, 'round'>>): string {
  const counts = new Map<number, number>();
  for (const play of plays) counts.set(play.round, (counts.get(play.round) ?? 0) + 1);
  return [...counts].map(([round, count]) => (count > 1 ? `R${round} ×${count}` : `R${round}`)).join(' · ');
}

export type SelectionCue = {
  play: Play;
  /** Estimated length of this highlight in the Short. */
  seconds: number;
  /** Estimated second at which it starts, in plan order. */
  startAt: number;
};

/** The Short as a running order: plan order, each cue with its estimated length and start. */
export function selectionTimeline(
  plays: readonly Play[],
  selection: ReplayOption & { selectedIds: ReadonlySet<string> },
): SelectionCue[] {
  const cues: SelectionCue[] = [];
  let elapsedTenths = 0;
  for (const play of plays) {
    if (!selection.selectedIds.has(play.id)) continue;
    const tenths = estimatedPlayTenths(play, selection);
    cues.push({ play, seconds: tenths / 10, startAt: elapsedTenths / 10 });
    elapsedTenths += tenths;
  }
  return cues;
}
