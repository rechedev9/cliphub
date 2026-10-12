import type { Play } from '../api/types.ts';

export type RoundOrdinal = { index: number; total: number };

/** Position of each play among the plays of its own round; only rounds with 2+ plays get an entry. */
export function roundOrdinals(plays: ReadonlyArray<Pick<Play, 'id' | 'round'>>): Map<string, RoundOrdinal> {
  const totals = new Map<number, number>();
  for (const play of plays) totals.set(play.round, (totals.get(play.round) ?? 0) + 1);
  const seen = new Map<number, number>();
  const ordinals = new Map<string, RoundOrdinal>();
  for (const play of plays) {
    const total = totals.get(play.round) ?? 1;
    const index = (seen.get(play.round) ?? 0) + 1;
    seen.set(play.round, index);
    if (total > 1) ordinals.set(play.id, { index, total });
  }
  return ordinals;
}

/** "Ronda 16" or, when the round has several plays, "Ronda 16 · 2/3". */
export function roundTitle(round: number, ordinal: RoundOrdinal | undefined): string {
  return ordinal ? `Ronda ${round} · ${ordinal.index}/${ordinal.total}` : `Ronda ${round}`;
}
