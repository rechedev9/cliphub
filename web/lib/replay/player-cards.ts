import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '../api/tactical.ts';
import type { TacticalRound, TacticalSample, TacticalSide } from '../api/tactical.ts';
import type { TacticalInventoryItem } from '../tactical-equipment.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';

/** Grenades a player has used so far in the round, by kind. */
export type UtilityUsed = { smoke: number; flash: number; he: number; molotov: number };

/**
 * One player at the playhead, from what the tactical document records: health
 * and the bomb from the position samples, money and equipment value from the
 * buy sample, kills, grenades and the last kill's weapon from the events so far.
 * Inventory, armour and the kit come from the samples when the parser records
 * them; without them the last kill's weapon is the only gun known.
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
  /** The round's assists once the playhead reaches its end; null before, because the document has no assist event. */
  assists: number | null;
  utility: UtilityUsed;
  lastWeapon: string | null;
  /** Null when the samples carry no gear (analyses before version 5). */
  inventory: readonly TacticalInventoryItem[] | null;
  activeWeapon: string | null;
  /** Seconds of flash blindness left; 0 when not blinded or unknown. */
  flashed: number;
  armor: number | null;
  helmet: boolean | null;
  defuser: boolean | null;
};

export type PlayerCardInput = {
  round: Pick<TacticalRound, 'players' | 'tick_end'>;
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
  const sideOf = new Map(input.round.players.map((player) => [player.slot, player.side]));

  for (const entry of input.events) {
    if (entry.seconds > input.nowSeconds) break;
    const { event } = entry;
    const actor = event.actor_slot;
    if (actor === undefined) continue;
    if (event.kind === TACTICAL_EVENT_KINDS.kill) {
      const victimSide = sideOf.get(event.target_slot ?? -1);
      if (victimSide !== undefined && victimSide === sideOf.get(actor)) continue;
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
      money: alive && sample?.money !== undefined ? sample.money : player.money,
      equipValue: player.equip_value,
      kills: kills.get(player.slot) ?? 0,
      assists: input.nowTick >= input.round.tick_end ? player.assists : null,
      utility: utility.get(player.slot) ?? emptyUtility(),
      lastWeapon: weapons.get(player.slot) ?? null,
      inventory: alive ? (sample?.inventory ?? null) : null,
      activeWeapon: alive ? (sample?.activeWeapon ?? null) : null,
      flashed: alive ? (sample?.flashed ?? 0) : 0,
      armor: alive ? (sample?.armor ?? null) : null,
      helmet: alive ? (sample?.helmet ?? null) : null,
      defuser: alive ? (sample?.defuser ?? null) : null,
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
