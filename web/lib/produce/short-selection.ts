import type { Play, VictimReplaySpeed } from '../api/types.ts';
import { clashingPlay } from './multi-pov.ts';

/** A Short should stay at or under one minute. */
export const SHORT_TARGET_SECONDS = 60;

/** Fallback for older plans without source timing. */
const BASE_SECONDS = 6;
const SECONDS_PER_KILL = 3;
type PlayTiming = Pick<Play, 'kills' | 'startSeconds' | 'endSeconds' | 'victimReplay'>;
type ReplayOption = { victimReplay?: boolean; victimReplaySpeed?: VictimReplaySpeed };

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
const VICTIM_REPLAY_CAPTURE_SECONDS = 3.5;

/** What a victim replay adds to each play at the chosen speed; 0 with the replay off. */
export function victimReplaySeconds(options: ReplayOption): number {
  if (!options.victimReplay) return 0;
  return options.victimReplaySpeed ? VICTIM_REPLAY_CAPTURE_SECONDS / options.victimReplaySpeed : VICTIM_REPLAY_SECONDS;
}

export function estimatedSelectionSeconds(plays: ReadonlyArray<PlayTiming>, options: ReplayOption = {}): number {
  return plays.reduce((total, play) => total + estimatedPlayTenths(play, options), 0) / 10;
}

/** Accumulate the displayed precision as integers so an exact minute stays within budget. */
function estimatedPlayTenths(play: PlayTiming, options: ReplayOption): number {
  const replay = play.victimReplay !== false ? victimReplaySeconds(options) : 0;
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
 * Greedy best-of: highest server score first (internal/moments), then kills,
 * then plan order, adding a
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
    .sort((a, b) => (b.play.score ?? -1) - (a.play.score ?? -1) || b.play.kills - a.play.kills || a.index - b.index);
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

type OverTargetInput = { totalSeconds: number; targetSeconds: number; /** What the victim replay adds to the total. */ replaySeconds: number; short: boolean };

/** Why the video runs past its target and what to do; names the victim replay when it is the cause. */
export function overTargetNotice({ totalSeconds, targetSeconds, replaySeconds, short }: OverTargetInput): string | null {
  if (totalSeconds <= targetSeconds) return null;
  const over = targetSeconds === SHORT_TARGET_SECONDS ? 'Pasa del minuto' : `Pasa de ${formatClock(targetSeconds)}`;
  const video = short ? 'un Short' : 'un vídeo';
  const replay = Math.round(replaySeconds);
  if (replay <= 0) return `${over}: quita una jugada o acepta ${video} más largo.`;
  if (totalSeconds - replaySeconds <= targetSeconds) {
    return `${over} por la repetición desde la víctima (+${replay} s): quítala, súbele la velocidad o quita una jugada.`;
  }
  return `${over}: la repetición desde la víctima suma ${replay} s. Quita una jugada o la repetición, o acepta ${video} más largo.`;
}

/** Kills and deaths keep their own selection: switching back restores the one left there. */
export type ReelSelections = { kills: readonly string[] | null; deaths: readonly string[] | null };

export function switchReelSelection(
  saved: ReelSelections,
  change: { current: readonly string[]; toDeaths: boolean; fallback: () => readonly string[] },
): { saved: ReelSelections; selectedIds: string[] } {
  const next: ReelSelections = change.toDeaths ? { ...saved, kills: change.current } : { ...saved, deaths: change.current };
  const restored = change.toDeaths ? saved.deaths : saved.kills;
  return { saved: next, selectedIds: [...(restored ?? change.fallback())] };
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
