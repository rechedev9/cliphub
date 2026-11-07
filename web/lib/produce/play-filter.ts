import type { Play, PlayPlayer, PlaySide } from '../api/types.ts';

export type PlayKindFilter = 'clutch' | 'ace';

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

/** A weapon keeps the plays where every kill used it; a kind or a side keeps the plays that have it. */
export function playMatches(play: Play, filter: PlayFilter): boolean {
  if (filter.side !== null && play.side !== filter.side) return false;
  if (filter.player !== null && play.player?.steamId !== filter.player) return false;
  if (filter.weapon !== null) {
    const weapons = play.weapons ?? [];
    if (weapons.length === 0 || weapons.some((weapon) => weapon !== filter.weapon)) return false;
  }
  if (filter.kind === 'clutch' && !(play.clutchVs && play.clutchVs > 0)) return false;
  if (filter.kind === 'ace' && play.kills < ACE_KILLS) return false;
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
  let clutch = false;
  let ace = false;
  for (const play of plays) {
    const used = play.weapons ?? [];
    // Only a weapon that can carry a whole play is worth offering.
    if (used.length === 1) weapons.set(used[0], (weapons.get(used[0]) ?? 0) + 1);
    if (play.side) sides.add(play.side);
    if (play.player && !players.some((p) => p.steamId === play.player?.steamId)) players.push(play.player);
    if (play.clutchVs && play.clutchVs > 0) clutch = true;
    if (play.kills >= ACE_KILLS) ace = true;
  }
  const kinds: PlayKindFilter[] = [];
  if (clutch) kinds.push('clutch');
  if (ace) kinds.push('ace');
  return {
    weapons: [...weapons.entries()].sort((a, b) => b[1] - a[1] || a[0].localeCompare(b[0])).map(([weapon]) => weapon),
    kinds,
    sides: (['CT', 'T'] as const).filter((side) => sides.has(side)),
    players,
  };
}
