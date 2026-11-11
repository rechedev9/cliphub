import type { TacticalEvent, TacticalFrame, TacticalSample } from '../api/tactical.ts';
import { sampleSetpos } from '../libreta/lineups.ts';
import { eventKindLabel } from '../tactical-labels.ts';
import { roundClockLabelFor, timelineSeconds } from '../tactical-timeline.ts';
import type { RoundTimeline } from '../tactical-timeline.ts';

/**
 * One setpos line copied from the Replay: whose position, at what round clock, and the console line.
 * `notice` is what the map says after the copy; without it the notice names the player.
 */
export type SetposCopy = { id: number; name: string; at: string; command: string; notice?: string };

/**
 * The recorded sample of a player nearest to `tick`: positions are sampled a few times a second and a
 * copy is never interpolated. The nearest frame that holds the player wins and a tie takes the earlier one,
 * as the MCP's player_setpos reads it.
 */
export function nearestSample(options: { frames: readonly TacticalFrame[]; slot: number; tick: number }): { tick: number; sample: TacticalSample } | null {
  let best: { tick: number; sample: TacticalSample } | null = null;
  for (const frame of options.frames) {
    const sample = frame.samples.find((candidate) => candidate.slot === options.slot);
    if (sample === undefined) continue;
    if (best === null || Math.abs(frame.tick - options.tick) < Math.abs(best.tick - options.tick)) best = { tick: frame.tick, sample };
  }
  return best;
}

function clockAt(options: { round: number; timeline: RoundTimeline; tick: number }): string {
  return `R${options.round} · ${roundClockLabelFor(options.timeline, timelineSeconds(options.timeline, options.tick))}`;
}

/**
 * What a click, S or a right-click on a player copies: their line from the recorded sample
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
  const found = nearestSample({ frames: options.frames, slot: options.slot, tick: options.tick });
  if (found === null) return null;
  const command = sampleSetpos(found.sample);
  if (command === null) return null;
  const name = options.players.find((player) => player.slot === options.slot)?.name ?? `Jugador ${options.slot}`;
  return { name, at: clockAt({ round: options.round, timeline: options.timeline, tick: found.tick }), command };
}

/**
 * What a click on a grenade's throw copies: where its thrower stood and looked on the tick it left
 * the hand, exactly as the analysis recorded it. Null for a grenade whose throw the demo never showed.
 */
export function grenadeSetpos(options: {
  event: TacticalEvent;
  players: readonly { slot: number; name: string }[];
  round: number;
  timeline: RoundTimeline;
}): Omit<SetposCopy, 'id'> | null {
  const { event } = options;
  const thrown = event.throw;
  if (thrown === undefined || event.actor_slot === undefined) return null;
  const command = sampleSetpos({ x: thrown.pos[0], y: thrown.pos[1], z: thrown.pos[2], yaw: thrown.yaw, pitch: thrown.pitch });
  if (command === null) return null;
  const slot = event.actor_slot;
  const name = options.players.find((player) => player.slot === slot)?.name ?? `Jugador ${slot}`;
  return {
    name,
    at: clockAt({ round: options.round, timeline: options.timeline, tick: thrown.tick }),
    command,
    notice: `Setpos del lanzamiento de ${name} (${eventKindLabel(event.kind)}) copiado`,
  };
}
