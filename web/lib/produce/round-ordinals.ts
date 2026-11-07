import type { Play } from '../api/types.ts';

export type RoundOrdinal = { index: number; total: number };

/**
 * Position of each play among the plays of its own round (and player, in a
 * multi-POV match); only groups with 2+ plays get an entry.
 */
export function roundOrdinals(plays: ReadonlyArray<Pick<Play, 'id' | 'round' | 'player'>>): Map<string, RoundOrdinal> {
  const key = (play: Pick<Play, 'round' | 'player'>): string => `${play.round}:${play.player?.steamId ?? ''}`;
  const totals = new Map<string, number>();
  for (const play of plays) totals.set(key(play), (totals.get(key(play)) ?? 0) + 1);
  const seen = new Map<string, number>();
  const ordinals = new Map<string, RoundOrdinal>();
  for (const play of plays) {
    const total = totals.get(key(play)) ?? 1;
    const index = (seen.get(key(play)) ?? 0) + 1;
    seen.set(key(play), index);
    if (total > 1) ordinals.set(play.id, { index, total });
  }
  return ordinals;
}

/** "Ronda 16" or, when the round has several plays, "Ronda 16 · 2/3". */
export function roundTitle(round: number, ordinal: RoundOrdinal | undefined): string {
  return ordinal ? `Ronda ${round} · ${ordinal.index}/${ordinal.total}` : `Ronda ${round}`;
}

/** The row title: "Ronda 16 · 2/3", led by the player in a multi-POV match ("ropz · Ronda 16"). */
export function playTitle(play: Pick<Play, 'round' | 'player'>, ordinal: RoundOrdinal | undefined): string {
  const round = roundTitle(play.round, ordinal);
  return play.player ? `${play.player.name} · ${round}` : round;
}
