import { TACTICAL_CT_PATTERN_ORDER, TACTICAL_T_PATTERN_ORDER } from '../api/tactical.ts';
import type { TacticalCTPattern, TacticalFilter, TacticalRound, TacticalTPattern, TacticalTeam } from '../api/tactical.ts';
import { filterTacticalRounds } from '../tactical-filter.ts';

export type RoundTypeCount<P extends string> = { pattern: P; count: number; selected: boolean };

/**
 * The round types of the left list with how many rounds each would show. The
 * counts ignore the pattern filters themselves, so picking a second type adds
 * exactly its count. Types with no round and no selection are left out.
 */
export function roundTypeCounts(
  teams: readonly TacticalTeam[],
  filter: TacticalFilter,
  rounds: readonly TacticalRound[],
): { t: RoundTypeCount<TacticalTPattern>[]; ct: RoundTypeCount<TacticalCTPattern>[] } {
  const { t_patterns: tSelected = [], ct_patterns: ctSelected = [] } = filter;
  const forT = filterTacticalRounds(teams, { ...filter, t_patterns: undefined }, rounds);
  const forCt = filterTacticalRounds(teams, { ...filter, ct_patterns: undefined }, rounds);
  const t = TACTICAL_T_PATTERN_ORDER.map((pattern) => ({
    pattern,
    count: forT.filter((round) => round.class.t_side === pattern).length,
    selected: tSelected.includes(pattern),
  })).filter((entry) => entry.count > 0 || entry.selected);
  const ct = TACTICAL_CT_PATTERN_ORDER.map((pattern) => ({
    pattern,
    count: forCt.filter((round) => round.class.ct_side === pattern).length,
    selected: ctSelected.includes(pattern),
  })).filter((entry) => entry.count > 0 || entry.selected);
  return { t, ct };
}

/** Adds or removes one value of a multi-value filter field; empty becomes undefined. */
export function toggleValue<T extends string>(values: readonly T[] | undefined, value: T): T[] | undefined {
  const current = values ?? [];
  const next = current.includes(value) ? current.filter((entry) => entry !== value) : [...current, value];
  return next.length === 0 ? undefined : next;
}
