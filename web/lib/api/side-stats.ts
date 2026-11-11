import type { PlayerMapRoles } from './baseline.ts';

// Browser-side contract of `tacticalplan.SideStatsReport` (`GET /api/library/teams/{id}/side-stats`),
// with the Go JSON names.

/** One figure; `value` and `group_avg` are null without data. Rates are percents. */
export type SideStat = {
  value: number | null;
  count: number;
  total: number;
  group_avg: number | null;
  group_teams: number;
};

export const SIDE_STAT_KEYS = [
  'maps_won',
  'rounds_won',
  'pistols_won',
  'opening_kills',
  'won_5v4',
  'won_4v5',
  'trades',
  'utility_damage_per_round',
  'flash_assists_per_round',
  'rounds_played',
] as const;
export type SideStatKey = (typeof SIDE_STAT_KEYS)[number];

export type SideStats = Record<SideStatKey, SideStat>;

export type SideStatsPlayer = {
  steamid64: string;
  name: string;
  role: string;
  /** The role per side on the map, as the Ranking reads it; set when the scope reads one map. */
  map_roles?: PlayerMapRoles;
  rounds: number;
  kills: number;
  deaths: number;
  damage: number;
  rating: number;
  kd: number;
  adr: number;
  kpr: number;
  dpr: number;
  /** Percent of rounds with a kill, an assist, survival or a trade. */
  kast: number;
  /** Per round over the rounds scanned with utility effects; null when none was. */
  udr: number | null;
  far: number | null;
  /** Percent of kills that were headshots; null without kills. */
  hs: number | null;
};

/** The Rendimiento de jugadores footer: the team's players pooled, or the group's mean of that line. */
export type PlayerLine = {
  rating: number | null;
  kpr: number | null;
  kd: number | null;
  dpr: number | null;
  adr: number | null;
  kast: number | null;
  udr: number | null;
  far: number | null;
  hs: number | null;
};

export type SideStatsMatchRef = { match_id: string; map: string; played_at: string; opponent: string; rounds: number; won: boolean };

export type TeamSideStats = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: string }[];
  compare: 'library' | 'teams';
  compare_teams: { team_id: string; name: string }[];
  /** Compare ids that are no library team, left out. */
  compare_skipped: string[];
  /** A group team had more matches than the per team cap, so only its newest count. */
  group_capped: boolean;
  /** Matches whose AWP filter was estimated from AWP kills: their analysis predates the AWP flag. */
  awp_estimated: number;
  group_matches: number;
  group_min_rounds: number;
  matches: number;
  rounds: number;
  overall: SideStats;
  ct: SideStats;
  t: SideStats;
  players: SideStatsPlayer[];
  players_average: PlayerLine;
  players_group_avg: PlayerLine;
  match_list: SideStatsMatchRef[];
  group_teams: number;
  rating_formula: string;
};

export const TEAM_SIDE_STATS_KEYS = [
  'team_id', 'team_name', 'skipped', 'compare', 'compare_teams', 'matches', 'rounds', 'overall', 'ct', 't', 'players',
  'match_list', 'group_teams', 'compare_skipped', 'group_capped', 'group_matches', 'group_min_rounds', 'rating_formula',
  'awp_estimated', 'players_average', 'players_group_avg',
] as const;
export const TEAM_SIDE_STATS_QUERY = [
  'map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup', 'compare_team',
] as const;
/** The server's cap on the teams of "Comparar con" (`httpapi.maxCompareTeams`). */
export const COMPARE_TEAMS_MAX = 20;
