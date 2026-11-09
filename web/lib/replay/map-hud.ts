import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '../api/tactical.ts';
import type { TacticalDocument, TacticalRound, TacticalSample, TacticalSide, TacticalSite } from '../api/tactical.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';

/** What the HUD over the map shows at the playhead: score bug, bomb timer and kill feed, as in the game. */

/** CS2's C4 timer from plant to explosion. */
export const BOMB_SECONDS = 40;
/** A kill stays in the feed this long, about as long as in the game. */
export const FEED_SECONDS = 7;
const FEED_MAX = 5;

export type BombState =
  | { kind: 'none' }
  | { kind: 'planted'; site: TacticalSite | null; remaining: number }
  | { kind: 'defused' }
  | { kind: 'exploded' };

/** The bomb at `now` (seconds on the round's own bar). */
export function bombState(events: readonly TimelineEvent[], now: number): BombState {
  let state: BombState = { kind: 'none' };
  for (const { event, seconds } of events) {
    if (seconds > now) break;
    if (event.kind === TACTICAL_EVENT_KINDS.plant) state = { kind: 'planted', site: event.site ?? null, remaining: Math.max(0, BOMB_SECONDS - (now - seconds)) };
    else if (event.kind === TACTICAL_EVENT_KINDS.defuse) state = { kind: 'defused' };
    else if (event.kind === TACTICAL_EVENT_KINDS.explode) state = { kind: 'exploded' };
  }
  return state;
}

/** From the plant to the defuse, the explosion or the end of the bar; null in a round without a plant. */
export function postPlantSpan(events: readonly TimelineEvent[], durationSeconds: number): { from: number; to: number } | null {
  const plant = events.find((entry) => entry.event.kind === TACTICAL_EVENT_KINDS.plant);
  if (plant === undefined) return null;
  const end = events.find((entry) => entry.seconds >= plant.seconds && (entry.event.kind === TACTICAL_EVENT_KINDS.defuse || entry.event.kind === TACTICAL_EVENT_KINDS.explode));
  const to = Math.min(durationSeconds, end?.seconds ?? durationSeconds);
  return to > plant.seconds ? { from: plant.seconds, to } : null;
}

/** The kills of the last few seconds, oldest first, as the game's feed stacks them. */
export function killFeed(events: readonly TimelineEvent[], now: number): TimelineEvent[] {
  const out = events.filter((entry) => entry.event.kind === TACTICAL_EVENT_KINDS.kill && entry.seconds <= now && now - entry.seconds < FEED_SECONDS);
  return out.slice(-FEED_MAX);
}

export type KillMark = { short: string; label: string };

/**
 * How a kill was made, as the feed marks it. The parser records all but the
 * air, which comes from the killer's sample at the kill: `airborne`.
 */
export function killMarks(event: TimelineEvent['event'], airborne: boolean): KillMark[] {
  const marks: [boolean | undefined, KillMark][] = [
    [event.headshot, { short: 'HS', label: 'Disparo a la cabeza' }],
    [event.wallbang, { short: 'WB', label: 'A través de una pared' }],
    [event.through_smoke, { short: 'Humo', label: 'A través de un humo' }],
    [event.no_scope, { short: 'Sin mira', label: 'Sin apuntar con la mira' }],
    [airborne, { short: 'Aire', label: 'Desde el aire' }],
    [event.attacker_blind, { short: 'Cegado', label: 'Con el tirador cegado' }],
  ];
  return marks.filter(([on]) => on === true).map(([, mark]) => mark);
}

/** Whether a slot was in the air in these samples, the frame of a kill. */
export function airborneIn(samples: readonly Pick<TacticalSample, 'slot' | 'flags'>[], slot: number | undefined): boolean {
  const sample = slot === undefined ? undefined : samples.find((entry) => entry.slot === slot);
  return sample !== undefined && hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.airborne);
}

/** Living players by side in one frame. */
export function aliveCounts(samples: readonly Pick<TacticalSample, 'flags'>[]): Record<TacticalSide, number> {
  const out: Record<TacticalSide, number> = { CT: 0, T: 0 };
  for (const sample of samples) {
    if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) continue;
    out[hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT) ? 'T' : 'CT'] += 1;
  }
  return out;
}

/** Each side's team name in this round: sides swap at the half, the players do not. */
export function teamNamesFor(doc: Pick<TacticalDocument, 'players' | 'teams'>, round: Pick<TacticalRound, 'players'>): Record<TacticalSide, string> {
  const teamOf = new Map(doc.players.map((player) => [player.slot, player.team_key]));
  const nameOf = new Map(doc.teams.map((team) => [team.key, team.name || team.key]));
  const out: Record<TacticalSide, string> = { CT: 'Antiterroristas', T: 'Terroristas' };
  for (const side of [TACTICAL_SIDES.ct, TACTICAL_SIDES.t]) {
    const player = round.players.find((entry) => entry.side === side);
    const key = player === undefined ? undefined : teamOf.get(player.slot);
    const name = key === undefined ? undefined : nameOf.get(key);
    if (name !== undefined) out[side] = name;
  }
  return out;
}

/** The round clock in whole seconds, as the HUD shows it: `1:07`, or `-0:12` during freeze time. */
export function hudClock(roundClockSeconds: number): string {
  if (!Number.isFinite(roundClockSeconds)) return '';
  const sign = roundClockSeconds < 0 ? '-' : '';
  const total = Math.floor(Math.abs(roundClockSeconds));
  return `${sign}${Math.floor(total / 60)}:${String(total % 60).padStart(2, '0')}`;
}
