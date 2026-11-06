import type { PlaybookCategoryName } from './playbook.ts';
import type { TacticalBuyType, TacticalRate, TacticalSide } from './tactical.ts';
import type { BombsiteMap } from './team-insights.ts';

/**
 * Browser-side contract of the library's team norms (`tacticalplan.Norms` plus
 * the handler's query): every team of every match pooled, both sides, with the
 * definitions of the team views. Field names are the Go JSON names.
 */

export type NormMatchup = { side: TacticalSide; buy: TacticalBuyType; opponent_buy: TacticalBuyType; win_rate: TacticalRate };

export type NormCategory = { category: PlaybookCategoryName; share: TacticalRate; win_rate: TacticalRate };

export type NormPlaybook = { side: TacticalSide; round_count: number; win_rate: TacticalRate; categories: NormCategory[] };

export type LibraryNorms = {
  /** Empty for every map. */
  map: string;
  from?: string;
  to?: string;
  matches: number;
  /** Matches whose analysis could not be read. */
  skipped: number;
  /** Examples are always empty: a norm names no round. */
  bombsites: BombsiteMap[];
  matchups: NormMatchup[];
  /** Both sides with a map; empty without one, since categories read one map's positions. */
  playbook: NormPlaybook[];
};

export type LibraryNormsQuery = { map?: string; from?: string; to?: string };

export const NORMS_KEYS = ['map', 'from', 'to', 'matches', 'skipped', 'bombsites', 'matchups', 'playbook'] as const;
export const NORMS_QUERY = ['map', 'from', 'to'] as const;
