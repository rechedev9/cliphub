import type { BaselineMetric } from './baseline.ts';

/**
 * Browser-side contract of the community endpoints (`httpapi/community.go`):
 * the rankings opt-in and the own player's place among every opted-in Studio.
 * Field names are the Go JSON names.
 */

export type CommunitySettings = {
  rankings_enabled: boolean;
  /** False when ZV_COMMUNITY_URL=off keeps Studio offline. */
  server_enabled: boolean;
  /** Rankings read the player of the SteamID set in Ajustes. */
  steam_id_set: boolean;
  last_submitted_at?: string;
  /** Set when turning rankings on could not submit: `steam_id_required` or `community_unavailable`. */
  submit_error?: string;
};

export type GlobalMetric = {
  metric: BaselineMetric;
  /** Null when the player is under the round minimum on this map. */
  value: number | null;
  installs: number;
  /** Share of installs this one beats, 0 to 100; null under the install minimum. */
  percentile: number | null;
  median: number | null;
};

export type GlobalRankings = {
  /** Empty for every map. */
  map: string;
  rankings_enabled: boolean;
  steam_id_set: boolean;
  min_installs: number;
  min_rounds: number;
  rounds: number;
  matches: number;
  metrics: GlobalMetric[];
};

export const COMMUNITY_SETTINGS_KEYS = ['rankings_enabled', 'server_enabled', 'steam_id_set', 'last_submitted_at', 'submit_error'] as const;
export const COMMUNITY_SETTINGS_FIELDS = ['rankings_enabled'] as const;
export const COMMUNITY_SUBMIT_KEYS = [...COMMUNITY_SETTINGS_KEYS, 'maps'] as const;
export const GLOBAL_RANKINGS_KEYS = ['map', 'rankings_enabled', 'steam_id_set', 'min_installs', 'min_rounds', 'rounds', 'matches', 'metrics'] as const;
export const GLOBAL_RANKINGS_QUERY = ['map'] as const;
