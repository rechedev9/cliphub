import type { Play, PlayPlayer, PlaySide } from '../api/types.ts';

/** Play kinds the list filters by, in the order the filter shows them; each but ace is a moments reason code. */
export const PLAY_KINDS = [
  'clutch', 'clutch_won', 'ace', 'fast_double', 'fast_triple', 'fast_quad', 'fast_ace', 'noscope', 'through_smoke', 'attacker_blind',
  'entry', 'retake', 'eco_win', 'flick',
] as const;
export type PlayKindFilter = (typeof PLAY_KINDS)[number];

/** Narrows the "Prepara tu Short" list; null keeps every play. Mirrors internal/moments.Filter. */
export type PlayFilter = {
  weapon: string | null;
  kind: PlayKindFilter | null;
  side: PlaySide | null;
  /** SteamID64 of one player of a multi-POV match. */
  player: string | null;
};

export const NO_PLAY_FILTER: PlayFilter = { weapon: null, kind: null, side: null, player: null };

const ACE_KILLS = 5;

export function isFilterActive(filter: PlayFilter): boolean {
  return filter.weapon !== null || filter.kind !== null || filter.side !== null || filter.player !== null;
}

/** Clutch and ace read the plan itself, so they work on plays without server reasons. */
function hasKind(play: Play, kind: PlayKindFilter): boolean {
  if (kind === 'clutch') return play.clutchVs !== undefined && play.clutchVs > 0;
  if (kind === 'ace') return play.kills >= ACE_KILLS;
  if (kind === 'clutch_won') return play.clutchWon === true;
  return play.reasons?.includes(kind) === true;
}

/** A weapon keeps the plays where every kill used it; a kind or a side keeps the plays that have it. */
export function playMatches(play: Play, filter: PlayFilter): boolean {
  if (filter.side !== null && play.side !== filter.side) return false;
  if (filter.player !== null && play.player?.steamId !== filter.player) return false;
  if (filter.weapon !== null) {
    const weapons = play.weapons ?? [];
    if (weapons.length === 0 || weapons.some((weapon) => weapon !== filter.weapon)) return false;
  }
  if (filter.kind !== null && !hasKind(play, filter.kind)) return false;
  return true;
}

export function filterPlays(plays: readonly Play[], filter: PlayFilter): Play[] {
  return isFilterActive(filter) ? plays.filter((play) => playMatches(play, filter)) : [...plays];
}

/** The filter choices this match can answer: only weapons, kinds and sides some play has. */
export type PlayFilterOptions = {
  weapons: string[];
  kinds: PlayKindFilter[];
  sides: PlaySide[];
  /** The players of a multi-POV match, in first-play order. */
  players: PlayPlayer[];
};

export function playFilterOptions(plays: readonly Play[]): PlayFilterOptions {
  const weapons = new Map<string, number>();
  const sides = new Set<PlaySide>();
  const players: PlayPlayer[] = [];
  for (const play of plays) {
    const used = play.weapons ?? [];
    // Only a weapon that can carry a whole play is worth offering.
    if (used.length === 1) weapons.set(used[0], (weapons.get(used[0]) ?? 0) + 1);
    if (play.side) sides.add(play.side);
    if (play.player && !players.some((p) => p.steamId === play.player?.steamId)) players.push(play.player);
  }
  return {
    weapons: [...weapons.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([weapon]) => weapon),
    kinds: PLAY_KINDS.filter((kind) => plays.some((play) => hasKind(play, kind))),
    sides: (['CT', 'T'] as const).filter((side) => sides.has(side)),
    players,
  };
}
