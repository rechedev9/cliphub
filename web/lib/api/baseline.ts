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

/** Metrics a leaderboard ranks on: the reference ones plus the rating (HLTV 2.0 approximation) and K/D. */
export const LEADERBOARD_METRICS = ['rating', 'kd', ...BASELINE_METRICS] as const;
export type LeaderboardMetric = (typeof LEADERBOARD_METRICS)[number];

/** Roles of a player on one map: CT anchors and rotators, T lurkers and packs. */
export const CT_ROLES = ['a_anchor', 'b_anchor', 'rotator'] as const;
export const T_ROLES = ['a_lurk', 'b_lurk', 'half_lurk', 'pack'] as const;
export type LeaderboardRole = (typeof CT_ROLES)[number] | (typeof T_ROLES)[number];

export type LeaderboardRoles = { ct?: LeaderboardRole; t?: LeaderboardRole };

/** A round of the row to open in the Replay (`LeaderboardRow.examples`). */
export type LeaderboardRound = { match_id: string; round: number; tick: number };

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
  /** The newest rounds in the filter, library rows only. */
  examples?: LeaderboardRound[];
};

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
};

export const BASELINE_KEYS = ['scope', 'map', 'source', 'description', 'updated_at', 'min_rounds', 'players', 'rounds', 'matches', 'skipped', 'metrics'] as const;

export const LEADERBOARD_KEYS = [
  'scope', 'map', 'source', 'description', 'updated_at', 'metric', 'higher_is_better', 'min_rounds', 'matches', 'rounds', 'skipped', 'players',
  'side', 'role', 'from', 'to', 'match_source',
] as const;

export const BASELINE_QUERY = ['scope', 'map', 'min_rounds'] as const;

export const LEADERBOARD_QUERY = [...BASELINE_QUERY, 'metric', 'limit', 'side', 'role', 'from', 'to', 'source'] as const;
