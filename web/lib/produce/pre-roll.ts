import { DEFAULT_PRE_ROLL_SECONDS, type EditConfig, type Play, type PreRollSeconds } from '../api/types.ts';

/** A one-player 16:9 summary gives each play more lead-in than a Short. */
export const SUMMARY_PRE_ROLL_SECONDS: PreRollSeconds = 5;

export function preRollOf(edit: Pick<EditConfig, 'preRollSeconds'>): PreRollSeconds {
  return edit.preRollSeconds ?? DEFAULT_PRE_ROLL_SECONDS;
}

/** The analysis cut is stored unset, as Studio reads it back. */
export function withPreRoll(edit: EditConfig, seconds: PreRollSeconds): EditConfig {
  const next: EditConfig = { ...edit };
  if (seconds === DEFAULT_PRE_ROLL_SECONDS) delete next.preRollSeconds;
  else next.preRollSeconds = seconds;
  return next;
}

type Recorded = { play: Play; members: Play[]; start: number | undefined; end: number | undefined };

/**
 * The plays the video records for this selection, as parser.WithPreRoll cuts them: each kill play starts
 * that long before its first kill, and a play of the same round and POV it then reaches joins the earlier one.
 * The round-start clamp is left out: freeze time keeps every kill further than 8 s from it.
 */
export function preRolledSelection(plays: readonly Play[], selection: { selectedIds: ReadonlySet<string>; preRollSeconds: PreRollSeconds }): Play[] {
  const { selectedIds, preRollSeconds } = selection;
  if (preRollSeconds === DEFAULT_PRE_ROLL_SECONDS) return plays.filter((play) => selectedIds.has(play.id));
  return recordedGroups(plays, preRollSeconds)
    .filter((group) => group.members.some((play) => selectedIds.has(play.id)))
    .map((group) => withWindow(group.play, group));
}

/** Every play with the window the video records when it is chosen; plays the pre-roll joins share one window. */
export function preRolledPlays(plays: readonly Play[], preRollSeconds: PreRollSeconds): Play[] {
  if (preRollSeconds === DEFAULT_PRE_ROLL_SECONDS) return [...plays];
  return recordedGroups(plays, preRollSeconds).flatMap((group) => group.members.map((play) => withWindow(play, group)));
}

function recordedGroups(plays: readonly Play[], preRollSeconds: PreRollSeconds): Recorded[] {
  const recorded: Recorded[] = [];
  for (const play of plays) {
    const start = play.firstKillSeconds === undefined ? play.startSeconds : play.firstKillSeconds - preRollSeconds;
    const last = recorded.at(-1);
    if (last !== undefined && joins(last, { play, start })) {
      last.members.push(play);
      last.end = Math.max(last.end ?? 0, play.endSeconds ?? 0);
      continue;
    }
    recorded.push({ play, members: [play], start, end: play.endSeconds });
  }
  return recorded;
}

function withWindow(play: Play, { start, end }: Recorded): Play {
  return start !== undefined && end !== undefined ? { ...play, startSeconds: Math.max(0, start), endSeconds: end } : { ...play };
}

function joins(last: Recorded, { play, start }: { play: Play; start: number | undefined }): boolean {
  return last.play.firstKillSeconds !== undefined && play.firstKillSeconds !== undefined && start !== undefined && last.end !== undefined
    && last.play.round === play.round && last.play.player?.steamId === play.player?.steamId && start <= last.end;
}
