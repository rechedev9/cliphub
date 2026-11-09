import { TACTICAL_EVENT_KINDS } from '../api/tactical.ts';
import type { TacticalEvent, TacticalHit } from '../api/tactical.ts';

/** How long a hit's line stays on the map, fading out. */
export const HIT_LINE_SECONDS = 1;

export type HitLineKind = 'bullet' | 'he' | 'fire' | 'flash';

/** A line from attacker to victim; `amount` is health lost, or seconds blinded for a flash. */
export type HitLine = { from: number; to: number; kind: HitLineKind; amount: number; headshot: boolean; alpha: number };

const FIRE_WEAPONS: ReadonlySet<string> = new Set(['molotov', 'incgrenade', 'inferno']);

export function hitKind(weapon: string | undefined): HitLineKind {
  if (weapon === 'hegrenade') return 'he';
  if (weapon !== undefined && FIRE_WEAPONS.has(weapon)) return 'fire';
  return 'bullet';
}

function fade(age: number): number | null {
  if (age < 0 || age > HIT_LINE_SECONDS) return null;
  return 1 - age / HIT_LINE_SECONDS;
}

/**
 * The damage and enemy flashes of the last second at `nowSeconds`, newest
 * last. A hidden player's lines go with them, as attacker or as victim.
 */
export function hitLines(options: {
  hits: readonly TacticalHit[];
  events: readonly TacticalEvent[];
  nowSeconds: number;
  secondsOf: (tick: number) => number;
  hiddenSlots: readonly number[];
}): HitLine[] {
  const { nowSeconds, secondsOf, hiddenSlots } = options;
  const shown = (from: number, to: number): boolean => !hiddenSlots.includes(from) && !hiddenSlots.includes(to);
  const lines: { at: number; line: HitLine }[] = [];
  for (const hit of options.hits) {
    const at = secondsOf(hit.tick);
    const alpha = fade(nowSeconds - at);
    if (alpha === null || !shown(hit.actor_slot, hit.target_slot)) continue;
    lines.push({ at, line: { from: hit.actor_slot, to: hit.target_slot, kind: hitKind(hit.weapon), amount: hit.damage, headshot: hit.headshot === true, alpha } });
  }
  for (const event of options.events) {
    if (event.kind !== TACTICAL_EVENT_KINDS.flash || event.actor_slot === undefined) continue;
    const at = secondsOf(event.tick);
    const alpha = fade(nowSeconds - at);
    if (alpha === null) continue;
    for (const blind of event.effect?.blinded ?? []) {
      if (!blind.enemy || !shown(event.actor_slot, blind.slot)) continue;
      lines.push({ at, line: { from: event.actor_slot, to: blind.slot, kind: 'flash', amount: blind.seconds, headshot: false, alpha } });
    }
  }
  return lines.sort((x, y) => x.at - y.at).map((entry) => entry.line);
}

/** How long a player's dot shows the damage they just took. */
export const HURT_SECONDS = 0.6;

/** Players hurt in the last `HURT_SECONDS`, each with the strength of its latest hit, 1 fading to 0. */
export function recentDamage(options: { hits: readonly TacticalHit[]; nowSeconds: number; secondsOf: (tick: number) => number }): Map<number, number> {
  const out = new Map<number, number>();
  for (const hit of options.hits) {
    const age = options.nowSeconds - options.secondsOf(hit.tick);
    if (age < 0 || age > HURT_SECONDS || hit.damage <= 0) continue;
    out.set(hit.target_slot, Math.max(out.get(hit.target_slot) ?? 0, 1 - age / HURT_SECONDS));
  }
  return out;
}
