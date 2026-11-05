import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '../api/tactical.ts';
import type { TacticalRound, TacticalSample, TacticalSide } from '../api/tactical.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';

/** Grenades a player has used so far in the round, by kind. */
export type UtilityUsed = { smoke: number; flash: number; he: number; molotov: number };

/**
 * One player at the playhead, from what the tactical document records: health
 * and the bomb from the position samples, money and equipment value from the
 * buy sample, kills, grenades and the last kill's weapon from the events so far.
 * The document carries no inventory, so the weapon is only known after a kill.
 */
export type PlayerCard = {
  slot: number;
  name: string;
  side: TacticalSide;
  alive: boolean;
  /** Null before the first position sample of a living player. */
  health: number | null;
  hasBomb: boolean;
  money: number;
  equipValue: number;
  kills: number;
  utility: UtilityUsed;
  lastWeapon: string | null;
};

export type PlayerCardInput = {
  round: Pick<TacticalRound, 'players'>;
  samples: readonly TacticalSample[];
  events: readonly TimelineEvent[];
  nowSeconds: number;
  nowTick: number;
  names: ReadonlyMap<number, string>;
};

function emptyUtility(): UtilityUsed {
  return { smoke: 0, flash: 0, he: 0, molotov: 0 };
}

/** Both sides' cards, CT first, each in the round's own player order so cards never jump. */
export function playerCards(input: PlayerCardInput): { ct: PlayerCard[]; t: PlayerCard[] } {
  const samples = new Map(input.samples.map((sample) => [sample.slot, sample]));
  const kills = new Map<number, number>();
  const weapons = new Map<number, string>();
  const utility = new Map<number, UtilityUsed>();

  for (const entry of input.events) {
    if (entry.seconds > input.nowSeconds) break;
    const { event } = entry;
    const actor = event.actor_slot;
    if (actor === undefined) continue;
    if (event.kind === TACTICAL_EVENT_KINDS.kill) {
      kills.set(actor, (kills.get(actor) ?? 0) + 1);
      if (event.weapon) weapons.set(actor, event.weapon);
      continue;
    }
    const used = utility.get(actor) ?? emptyUtility();
    if (event.kind === TACTICAL_EVENT_KINDS.smoke) used.smoke += 1;
    else if (event.kind === TACTICAL_EVENT_KINDS.flash) used.flash += 1;
    else if (event.kind === TACTICAL_EVENT_KINDS.he) used.he += 1;
    else if (event.kind === TACTICAL_EVENT_KINDS.molotov) used.molotov += 1;
    else continue;
    utility.set(actor, used);
  }

  const cards: PlayerCard[] = input.round.players.map((player) => {
    const sample = samples.get(player.slot);
    const alive = sample === undefined
      ? player.death_tick === undefined || input.nowTick < player.death_tick
      : hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive);
    let health: number | null = null;
    if (!alive) health = 0;
    else if (sample !== undefined) health = Math.max(0, Math.min(100, sample.health));
    return {
      slot: player.slot,
      name: input.names.get(player.slot) ?? `Jugador ${player.slot}`,
      side: player.side,
      alive,
      health,
      hasBomb: alive && sample !== undefined && hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb),
      money: player.money,
      equipValue: player.equip_value,
      kills: kills.get(player.slot) ?? 0,
      utility: utility.get(player.slot) ?? emptyUtility(),
      lastWeapon: weapons.get(player.slot) ?? null,
    };
  });
  return {
    ct: cards.filter((card) => card.side === TACTICAL_SIDES.ct),
    t: cards.filter((card) => card.side === TACTICAL_SIDES.t),
  };
}

/** `2 humos · 1 flash`, the grenades a card lists; empty when none was used. */
export function utilityLine(used: UtilityUsed): string {
  const parts: string[] = [];
  if (used.smoke > 0) parts.push(`${used.smoke} ${used.smoke === 1 ? 'humo' : 'humos'}`);
  if (used.flash > 0) parts.push(`${used.flash} ${used.flash === 1 ? 'flash' : 'flashes'}`);
  if (used.he > 0) parts.push(`${used.he} HE`);
  if (used.molotov > 0) parts.push(`${used.molotov} ${used.molotov === 1 ? 'molotov' : 'molotovs'}`);
  return parts.join(' · ');
}
