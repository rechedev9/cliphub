import type { TeamGrenade } from '../api/grenades.ts';
import { PRACTICE_MAX_SPOTS, type PracticeGrenade, type PracticeSpot } from '../api/practice.ts';
import { GRENADE_KIND_LABEL, type GrenadeSpot } from './grenade-patterns.ts';

/**
 * Turns throws into CS2 practice spots: where the thrower stood (feet, which
 * is what `setpos` takes), where they looked, and which grenade to hand them.
 */

/** Why a throw cannot be practised, in the words the button shows. */
export const NO_ORIGIN_REASON = 'Esta demo no guarda desde dónde se lanzó. Studio la vuelve a analizar al arrancar.';

/** CT throws an incendiary, T a molotov; CS2 gives them as different weapons. */
function practiceGrenade(grenade: TeamGrenade): PracticeGrenade {
  if (grenade.kind === 'molotov' && grenade.side === 'CT') return 'incendiary';
  return grenade.kind;
}

/** True when the throw carries what a practice spot needs. */
export function canPractice(grenade: TeamGrenade): boolean {
  return grenade.origin !== null && grenade.view !== null;
}

/** A practice spot for one throw, or null when the demo did not record the throw position. */
export function practiceSpotFor(grenade: TeamGrenade, label: string): PracticeSpot | null {
  if (grenade.origin === null || grenade.view === null) return null;
  const [yaw, pitch] = grenade.view;
  return {
    label: label.slice(0, 200),
    pos: [grenade.origin[0], grenade.origin[1], grenade.origin[2]],
    ang: [pitch, yaw],
    grenade: practiceGrenade(grenade),
  };
}

/** The real throw of a spot closest to its centre, so the angles belong to a throw that happened. */
export function representativeThrow(spot: GrenadeSpot): TeamGrenade | null {
  const centre = spot.origin;
  let best: TeamGrenade | null = null;
  let bestDistance = Infinity;
  for (const grenade of spot.grenades) {
    if (!canPractice(grenade) || grenade.origin === null || centre === null) continue;
    const distance = Math.hypot(grenade.origin[0] - centre[0], grenade.origin[1] - centre[1]);
    if (distance < bestDistance) {
      best = grenade;
      bestDistance = distance;
    }
  }
  return best;
}

/** "Humo a Ventana desde T Spawn", with whichever ends have a callout, each named by `place`. */
export function spotLabel(
  kind: GrenadeSpot['kind'],
  ends: { landing: string | null; origin: string | null },
  place: (token: string) => string,
): string {
  let label = GRENADE_KIND_LABEL[kind];
  if (ends.landing !== null) label += ` a ${place(ends.landing)}`;
  if (ends.origin !== null) label += ` desde ${place(ends.origin)}`;
  return label;
}

/** Practice spots for the chosen groups, in order, at most what one cfg holds. */
export function practiceSpotsForSpots(spots: readonly { spot: GrenadeSpot; label: string }[]): PracticeSpot[] {
  const out: PracticeSpot[] = [];
  for (const { spot, label } of spots) {
    if (out.length >= PRACTICE_MAX_SPOTS) break;
    const grenade = representativeThrow(spot);
    const practice = grenade === null ? null : practiceSpotFor(grenade, label);
    if (practice !== null) out.push(practice);
  }
  return out;
}
