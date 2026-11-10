/**
 * Browser-side contract of the reference values and the local ranking
 * (`tacticalplan.Baseline` and `LeaderboardRow` plus the handler's scope).
 * Field names are the Go JSON names.
 */

export const BASELINE_METRICS = [
  'kpr',
  'dpr',
  'adr',
  'kast',
  'opening_win_pct',
  'trade_pct',
  'utility_damage_per_round',
  'flash_assists_per_round',
  'wpa_per_round',
] as const;
export type BaselineMetric = (typeof BASELINE_METRICS)[number];

/** `library` reads every match; `pro` the library's HLTV demos, or the bundled pro reference when they hold too few players. */
export const BASELINE_SCOPES = ['library', 'pro'] as const;
export type BaselineScope = (typeof BASELINE_SCOPES)[number];

/** Where a reference's numbers come from: the library, the pro reference Studio ships, or the one the ClipHub server measures. */
export type BaselineSource = 'library' | 'bundled' | 'server';

/**
 * The mean of one metric over the qualifying players that have it, with the
 * spread of their true values (`prior_stddev`, sampling noise removed) and,
 * for a per-round rate, one round's spread.
 */
export type BaselineValue = { mean: number; players: number; stddev: number; prior_stddev: number; round_stddev?: number };

export type Baseline = {
  scope: BaselineScope;
  /** The map the numbers read; empty for every map, also when the bundled reference has no entry for the map asked for. */
  map: string;
  source: BaselineSource;
  /** Names the matches of the bundled or server reference; absent for the library. */
  description?: string;
  /** When the server reference was measured, RFC 3339; absent otherwise. */
  updated_at?: string;
  min_rounds: number;
  /** Players with at least `min_rounds` rounds. */
  players: number;
  rounds: number;
  matches: number;
  /** Matches whose analysis could not be read. */
  skipped: number;
  metrics: Record<BaselineMetric, BaselineValue>;
};

/** Metrics a leaderboard ranks on: the reference ones plus ClipHub's own rating and K/D. */
export const LEADERBOARD_METRICS = ['rating', 'kd', ...BASELINE_METRICS] as const;
export type LeaderboardMetric = (typeof LEADERBOARD_METRICS)[number];

/** Roles of a player on one map: CT anchors and rotators, T lurkers and packs. */
export const CT_ROLES = ['a_anchor', 'b_anchor', 'rotator'] as const;
export const T_ROLES = ['a_lurk', 'b_lurk', 'half_lurk', 'pack'] as const;
export type LeaderboardRole = (typeof CT_ROLES)[number] | (typeof T_ROLES)[number];

export type LeaderboardRoles = { ct?: LeaderboardRole; t?: LeaderboardRole };

/** A round of the row to open in the Replay (`LeaderboardRow.examples`). */
export type LeaderboardRound = { match_id: string; round: number; tick: number };

/** A role or a position and the share of the player's rounds it covers, in percent. */
export type BreakdownShare = { key: string; label?: string; pct: number };

/** How a player's rounds on one side split between roles and between positions (callouts). */
export type SideBreakdown = { role?: LeaderboardRole; roles: BreakdownShare[]; map_role?: string; places: BreakdownShare[] };

/** A position players hold on the map and side, with how many. */
export type MapRoleOption = { key: string; label: string; players: number };

export type LeaderboardRow = {
  rank: number;
  steamid64: string;
  name: string;
  /** Set on the bundled pro rows, which have no player page. */
  team?: string;
  rounds: number;
  matches: number;
  value: number;
  /** Every leaderboard metric the player has; library rows only. */
  metrics?: Partial<Record<LeaderboardMetric, number>>;
  /** The role per side on the map; present when the query names a map. */
  roles?: LeaderboardRoles;
  /** Roles and positions as shares of the player's rounds per side; present when the query names a map. */
  breakdown?: { ct?: SideBreakdown; t?: SideBreakdown };
  /** The rounds of the filtered role or position, else the newest of the side; library rows only. */
  examples?: LeaderboardRound[];
};

/** Team levels of the pro board: the HLTV world rank of the rival or of the player's own team. */
export const PRO_TOPS = [5, 15, 30, 50] as const;
export type ProTop = (typeof PRO_TOPS)[number];

/** Where demos come from: the Valve matchmaking, FACEIT, tournaments or scrims. */
export const LEADERBOARD_SOURCES = ['valve', 'faceit', 'hltv', 'scrim'] as const;
export type LeaderboardMatchSource = (typeof LEADERBOARD_SOURCES)[number];

export type Leaderboard = {
  scope: BaselineScope;
  map: string;
  source: BaselineSource;
  description?: string;
  updated_at?: string;
  metric: LeaderboardMetric;
  /** False for `dpr`, where the lowest value ranks first. */
  higher_is_better: boolean;
  min_rounds: number;
  matches: number;
  rounds: number;
  skipped: number;
  /** Pro scope: the period the board read, in days. */
  days?: number;
  /** Positions players hold on the filtered map and side, to pick `map_role` from. */
  map_roles?: MapRoleOption[];
  /** Pro events in the period, to pick `event` from. */
  events?: string[];
  players: LeaderboardRow[];
};

export type BaselineQuery = { scope?: BaselineScope; map?: string; min_rounds?: number };

/** The filters beyond `scope`, `map` and `min_rounds` apply to the library scope only. */
export type LeaderboardQuery = BaselineQuery & {
  metric: LeaderboardMetric;
  limit?: number;
  side?: 'CT' | 'T';
  role?: LeaderboardRole;
  /** RFC 3339 bounds of the match date. */
  from?: string;
  to?: string;
  source?: LeaderboardMatchSource;
  /** A callout, or `allround`, held on `side`; needs the map and the side. */
  map_role?: string;
  /** Pro scope: only maps from the last N days (`all` for every date, dated or not), of one event, against a team ranked `top` or better, and players whose own team is ranked `team_top` or better. */
  days?: number | 'all';
  top?: ProTop;
  team_top?: ProTop;
  event?: string;
};

export const BASELINE_KEYS = ['scope', 'map', 'source', 'description', 'updated_at', 'min_rounds', 'players', 'rounds', 'matches', 'skipped', 'metrics'] as const;

export const LEADERBOARD_KEYS = [
  'scope', 'map', 'source', 'description', 'updated_at', 'metric', 'higher_is_better', 'min_rounds', 'matches', 'rounds', 'skipped', 'players',
  'side', 'role', 'from', 'to', 'match_source', 'days', 'top', 'team_top', 'event', 'map_role', 'map_roles', 'events',
] as const;

export const BASELINE_QUERY = ['scope', 'map', 'min_rounds'] as const;

export const LEADERBOARD_QUERY = [...BASELINE_QUERY, 'metric', 'limit', 'side', 'role', 'from', 'to', 'source', 'days', 'top', 'team_top', 'event', 'map_role'] as const;
