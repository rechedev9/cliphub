import type { Match } from '../api/types.ts';

export const MATCH_SOURCE_LABEL = { steam: 'Steam', upload: 'Importada' } as const;

/** Where a partida's demo came from, as the library names it. */
export function matchSourceLabel(match: Pick<Match, 'source'>): string {
  return match.source === undefined ? '' : MATCH_SOURCE_LABEL[match.source];
}

/** Epoch ms of the partida for sorting; an unreadable date sorts last. */
export function matchPlayedAtValue(match: Pick<Match, 'playedAt'>): number | null {
  const value = Date.parse(match.playedAt);
  return Number.isNaN(value) ? null : value;
}
