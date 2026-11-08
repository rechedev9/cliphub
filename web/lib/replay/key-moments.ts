import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, TACTICAL_SITES, hasSampleFlags } from '../api/tactical.ts';
import type { TacticalEvent, TacticalFrame, TacticalRound, TacticalSide } from '../api/tactical.ts';
import { siteLabel } from '../tactical-labels.ts';

/** Same window as the scan's trades and KAST. */
const TRADE_WINDOW_SECONDS = 5;
/** A defuse started again within this long is the same attempt, not a new one. */
const DEFUSE_REPEAT_SECONDS = 1;

export type KeyMomentKind = 'opening' | 'trade' | 'plant' | 'defuse' | 'clutch';

/** `label` names who did it; `KEY_MOMENT_LABELS` names what it was. */
export type KeyMoment = { tick: number; kind: KeyMomentKind; label: string; side: TacticalSide | null };

export const KEY_MOMENT_LABELS: Record<KeyMomentKind, string> = {
  opening: 'Primera baja',
  trade: 'Intercambio',
  plant: 'Bomba plantada',
  defuse: 'Empieza a desactivar',
  clutch: 'Clutch',
};

function otherSide(side: TacticalSide): TacticalSide {
  return side === TACTICAL_SIDES.ct ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
}

function kills(round: Pick<TacticalRound, 'events'>): TacticalEvent[] {
  return round.events.filter((event) => event.kind === TACTICAL_EVENT_KINDS.kill && event.target_slot !== undefined);
}

/** A kill of the player who killed a teammate of the killer inside the trade window. */
function isTrade(kill: TacticalEvent, earlier: readonly TacticalEvent[], window: number): boolean {
  if (kill.actor_slot === undefined) return false;
  return earlier.some((prev) => prev.actor_slot === kill.target_slot && prev.side !== kill.side && kill.tick - prev.tick <= window);
}

function clutchStart(options: { round: Pick<TacticalRound, 'events' | 'players'>; name: (slot: number) => string }): KeyMoment | null {
  const sideOf = new Map(options.round.players.map((player) => [player.slot, player.side]));
  const alive = new Map<TacticalSide, Set<number>>([
    [TACTICAL_SIDES.ct, new Set()],
    [TACTICAL_SIDES.t, new Set()],
  ]);
  for (const player of options.round.players) alive.get(player.side)?.add(player.slot);
  for (const kill of kills(options.round)) {
    const victim = kill.target_slot ?? -1;
    const side = sideOf.get(victim);
    if (side !== undefined) alive.get(side)?.delete(victim);
    for (const lone of [TACTICAL_SIDES.ct, TACTICAL_SIDES.t]) {
      const own = alive.get(lone) ?? new Set<number>();
      const rivals = alive.get(otherSide(lone))?.size ?? 0;
      const [last] = own;
      if (own.size !== 1 || rivals === 0 || last === undefined) continue;
      return { tick: kill.tick, kind: 'clutch', label: `${options.name(last)}, 1 contra ${rivals}`, side: lone };
    }
  }
  return null;
}

function defuseStarts(frames: readonly TacticalFrame[], options: { tickrate: number; name: (slot: number) => string }): KeyMoment[] {
  const out: KeyMoment[] = [];
  const defusing = new Set<number>();
  const lastStart = new Map<number, number>();
  for (const frame of frames) {
    for (const sample of frame.samples) {
      const now = hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.defusing);
      if (now && !defusing.has(sample.slot)) {
        const previous = lastStart.get(sample.slot);
        if (previous === undefined || frame.tick - previous > DEFUSE_REPEAT_SECONDS * options.tickrate) {
          out.push({ tick: frame.tick, kind: 'defuse', label: options.name(sample.slot), side: TACTICAL_SIDES.ct });
        }
        lastStart.set(sample.slot, frame.tick);
      }
      if (now) defusing.add(sample.slot);
      else defusing.delete(sample.slot);
    }
  }
  return out;
}

/**
 * The moments worth jumping to in a round, in tick order: the opening kill,
 * each trade kill, the plant, each defuse attempt and the start of a clutch.
 * Nothing after `endTick` counts: the kills of the post-round decide nothing.
 */
export function keyMoments(options: {
  round: Pick<TacticalRound, 'events' | 'players'>;
  frames: readonly TacticalFrame[];
  tickrate: number;
  names: ReadonlyMap<number, string>;
  endTick?: number;
}): KeyMoment[] {
  const { tickrate } = options;
  const endTick = options.endTick ?? Number.POSITIVE_INFINITY;
  const round = { players: options.round.players, events: options.round.events.filter((event) => event.tick <= endTick) };
  const frames = options.frames.filter((frame) => frame.tick <= endTick);
  const name = (slot: number | undefined): string => (slot === undefined ? 'El mundo' : (options.names.get(slot) ?? `Jugador ${slot + 1}`));
  const out: KeyMoment[] = [];
  const window = TRADE_WINDOW_SECONDS * tickrate;
  const seen: TacticalEvent[] = [];
  for (const kill of kills(round)) {
    const victim = name(kill.target_slot);
    if (kill.opening === true) out.push({ tick: kill.tick, kind: 'opening', label: `${name(kill.actor_slot)} mata a ${victim}`, side: kill.side ?? null });
    else if (isTrade(kill, seen, window)) out.push({ tick: kill.tick, kind: 'trade', label: `${name(kill.actor_slot)} mata a ${victim}`, side: kill.side ?? null });
    seen.push(kill);
  }
  for (const event of round.events) {
    if (event.kind !== TACTICAL_EVENT_KINDS.plant) continue;
    const site = event.site === undefined || event.site === TACTICAL_SITES.none ? '' : ` en ${siteLabel(event.site)}`;
    out.push({ tick: event.tick, kind: 'plant', label: `${name(event.actor_slot)}${site}`, side: TACTICAL_SIDES.t });
  }
  out.push(...defuseStarts(frames, { tickrate, name }));
  const clutch = clutchStart({ round, name });
  if (clutch !== null) out.push(clutch);
  return out.sort((a, b) => a.tick - b.tick);
}
