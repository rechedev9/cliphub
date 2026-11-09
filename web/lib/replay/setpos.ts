import type { TacticalFrame } from '../api/tactical.ts';
import { sampleSetpos } from '../libreta/lineups.ts';
import { frameCursor } from '../tactical-replay.ts';
import { roundClockLabelFor, timelineSeconds } from '../tactical-timeline.ts';
import type { RoundTimeline } from '../tactical-timeline.ts';

/** One setpos line copied from the Replay: whose position, at what round clock, and the console line. */
export type SetposCopy = { id: number; name: string; at: string; command: string };

/** How many copied lines the player panel keeps. */
const SETPOS_KEPT = 6;

/** The list with a new copy on top; a line already there moves up instead of repeating. */
export function withSetpos(list: readonly SetposCopy[], entry: Omit<SetposCopy, 'id'>): SetposCopy[] {
  const id = list.reduce((top, item) => Math.max(top, item.id), 0) + 1;
  return [{ ...entry, id }, ...list.filter((item) => item.command !== entry.command)].slice(0, SETPOS_KEPT);
}

/**
 * What S or a right-click on a player copies: their line from the recorded sample
 * nearest to `tick`, as the MCP's player_setpos reads it. z is the feet origin, as in the lineups.
 */
export function playerSetpos(options: {
  frames: readonly TacticalFrame[];
  players: readonly { slot: number; name: string }[];
  round: number;
  timeline: RoundTimeline;
  slot: number;
  tick: number;
}): Omit<SetposCopy, 'id'> | null {
  const { frames, timeline, slot } = options;
  const cursor = frameCursor(frames, options.tick);
  if (cursor === undefined) return null;
  // Past halfway the next sample is nearer; the earlier one stands in when it lacks the player.
  const order = cursor.alpha > 0.5 ? [cursor.index + 1, cursor.index] : [cursor.index];
  for (const index of order) {
    const frame = frames[index];
    const sample = frame?.samples.find((candidate) => candidate.slot === slot);
    if (frame === undefined || sample === undefined) continue;
    const name = options.players.find((player) => player.slot === slot)?.name ?? `Jugador ${slot}`;
    const at = `R${options.round} · ${roundClockLabelFor(timeline, timelineSeconds(timeline, frame.tick))}`;
    return { name, at, command: sampleSetpos(sample) };
  }
  return null;
}
