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

/** Where a reference's numbers come from: the library, or the pro reference Studio ships. */
export type BaselineSource = 'library' | 'bundled';

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
  /** Names the matches of the bundled reference; absent for the library. */
  description?: string;
  min_rounds: number;
  /** Players with at least `min_rounds` rounds. */
  players: number;
  rounds: number;
  matches: number;
  /** Matches whose analysis could not be read. */
  skipped: number;
  metrics: Record<BaselineMetric, BaselineValue>;
};

export type LeaderboardRow = {
  rank: number;
  steamid64: string;
  name: string;
  /** Set on the bundled pro rows, which have no player page. */
  team?: string;
  rounds: number;
  matches: number;
  value: number;
};

export type Leaderboard = {
  scope: BaselineScope;
  map: string;
  source: BaselineSource;
  description?: string;
  metric: BaselineMetric;
  /** False for `dpr`, where the lowest value ranks first. */
  higher_is_better: boolean;
  min_rounds: number;
  matches: number;
  rounds: number;
  skipped: number;
  players: LeaderboardRow[];
};

export type BaselineQuery = { scope?: BaselineScope; map?: string; min_rounds?: number };

export type LeaderboardQuery = BaselineQuery & { metric: BaselineMetric; limit?: number };

export const BASELINE_KEYS = ['scope', 'map', 'source', 'description', 'min_rounds', 'players', 'rounds', 'matches', 'skipped', 'metrics'] as const;

export const LEADERBOARD_KEYS = [
  'scope', 'map', 'source', 'description', 'metric', 'higher_is_better', 'min_rounds', 'matches', 'rounds', 'skipped', 'players',
] as const;

export const BASELINE_QUERY = ['scope', 'map', 'min_rounds'] as const;

export const LEADERBOARD_QUERY = [...BASELINE_QUERY, 'metric', 'limit'] as const;
