import type { Match } from '../api/types.ts';
import { prettyMapName } from '../format.ts';

export const MATCH_SOURCE_LABEL = { steam: 'Steam', upload: 'Importada' } as const;

/** Where a partida's demo came from, as the library names it. */
export function matchSourceLabel(match: Pick<Match, 'source'>): string {
  return match.source === undefined ? '' : MATCH_SOURCE_LABEL[match.source];
}

/** One filter option per map in the library, by display name. */
export function mapFilterOptions(matches: readonly Pick<Match, 'map'>[]): { value: string; label: string }[] {
  const maps = new Set(matches.map((match) => match.map).filter((map) => map !== ''));
  return [...maps]
    .map((map) => ({ value: map, label: prettyMapName(map) }))
    .sort((a, b) => a.label.localeCompare(b.label, 'es'));
}

/** Epoch ms of the partida for sorting; an unreadable date sorts last. */
export function matchPlayedAtValue(match: Pick<Match, 'playedAt'>): number | null {
  const value = Date.parse(match.playedAt);
  return Number.isNaN(value) ? null : value;
}
