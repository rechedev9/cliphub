import type {
  TacticalBuyType,
  TacticalEventKind,
  TacticalHistogram,
  TacticalRate,
  TacticalSide,
  TacticalSite,
  TacticalTendencies,
} from './tactical.ts';

/**
 * Browser-side contract of the scouting report (`tacticalplan.ScoutReport`
 * plus the handler's team and skip list). Field names are the Go JSON names.
 */

/** One round a number counted, by match, round and tick (`tacticalplan.Evidence`). */
export type ScoutEvidence = { match_id: string; round: number; tick: number };

/** A set of rounds, its share of the section, its wins and its first rounds. */
export type ScoutRounds = {
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
  examples: ScoutEvidence[];
};

export type ScoutPatternRow = ScoutRounds & { side: TacticalSide; pattern: string; site: TacticalSite };

export type ScoutSideRow = ScoutRounds & { side: TacticalSide };

export type ScoutPlayerCount = { steamid64: string; name: string; count: number };

export type ScoutUtilityRow = {
  kind: TacticalEventKind;
  callout: string;
  rounds: number;
  share: TacticalRate;
  count: number;
  /** When it landed, in seconds from freeze end. */
  median_seconds: number;
  throwers: ScoutPlayerCount[];
  examples: ScoutEvidence[];
};

export type ScoutPlaceRow = {
  place: string;
  rounds: number;
  share: TacticalRate;
  won: TacticalRate;
  examples: ScoutEvidence[];
};

export type ScoutDefaults = ScoutRounds & {
  patterns: ScoutPatternRow[];
  utility: ScoutUtilityRow[];
  opening_places: ScoutPlaceRow[];
  first_contact: TacticalHistogram;
  utility_round_total: number;
};

export type ScoutSpot = { callout: string; share: TacticalRate; examples: ScoutEvidence[] };

export type ScoutPositionRow = { steamid64: string; name: string; rounds: number; spots: ScoutSpot[] };

export type ScoutSetups = ScoutRounds & {
  patterns: ScoutPatternRow[];
  utility: ScoutUtilityRow[];
  /** Rounds with a position frame to read the setup from. */
  sampled: number;
  positions: ScoutPositionRow[];
};

export type ScoutExecute = ScoutRounds & {
  site: TacticalSite;
  patterns: ScoutPatternRow[];
  plants: TacticalRate;
  plant_time: TacticalHistogram;
  first_contact: TacticalHistogram;
  mean_utility: Partial<Record<TacticalEventKind, number>>;
  utility: ScoutUtilityRow[];
};

export const SCOUT_ECONOMY_CONTEXTS = ['after_pistol_loss', 'after_loss'] as const;
export type ScoutEconomyContextName = (typeof SCOUT_ECONOMY_CONTEXTS)[number];

export type ScoutEconomyContext = ScoutRounds & { context: ScoutEconomyContextName; side: TacticalSide };

export type ScoutEconomyRow = ScoutRounds & { context: ScoutEconomyContextName; side: TacticalSide; buy: TacticalBuyType };

export type ScoutAWPSide = {
  side: TacticalSide;
  rounds: number;
  kill_rounds: TacticalRate;
  kills: number;
  opening_kills: number;
};

export type ScoutAWPRow = {
  side: TacticalSide;
  place: string;
  kills: number;
  share: TacticalRate;
  opening_kills: number;
  players: ScoutPlayerCount[];
  examples: ScoutEvidence[];
};

export type ScoutOpening = {
  steamid64: string;
  name: string;
  side: TacticalSide;
  rounds: number;
  attempts: TacticalRate;
  won: TacticalRate;
  traded_on_loss: TacticalRate;
  places: { place: string; count: number }[];
  examples: ScoutEvidence[];
};

export type ScoutMatchSummary = {
  match_id: string;
  map: string;
  played_at: string;
  team_key: string;
  opponent_id?: string;
  opponent: string;
  score: number;
  opponent_score: number;
  rounds: number;
  wins: number;
};

export type ScoutMapSummary = {
  map: string;
  matches: number;
  match_wins: number;
  rounds: number;
  win_rate: TacticalRate;
  ct_win_rate: TacticalRate;
  t_win_rate: TacticalRate;
};

export type ScoutThresholds = {
  min_reliable_sample: number;
  example_limit: number;
  early_utility_seconds: number;
  execute_window_seconds: number;
  setup_seconds: number;
  utility_row_limit: number;
};

export type ScoutFilter = {
  maps?: string[];
  side?: TacticalSide;
  from?: string;
  to?: string;
  opponents?: string[];
};

export const SCOUT_SKIP_REASONS = ['not_found', 'team_not_in_match', 'analysis_missing', 'over_match_limit'] as const;
export type ScoutSkipReason = (typeof SCOUT_SKIP_REASONS)[number];

export type ScoutReport = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  filter: ScoutFilter;
  thresholds: ScoutThresholds;
  matches: ScoutMatchSummary[];
  maps: ScoutMapSummary[];
  tendencies: TacticalTendencies;
  pistols: { sides: ScoutSideRow[]; rows: ScoutPatternRow[] };
  t_defaults: ScoutDefaults;
  ct_setups: ScoutSetups;
  executes: ScoutExecute[];
  economy: { contexts: ScoutEconomyContext[]; rows: ScoutEconomyRow[] };
  awp: { sides: ScoutAWPSide[]; rows: ScoutAWPRow[] };
  openings: ScoutOpening[];
};

/** "" reads both sides. */
export type ReportSide = '' | TacticalSide;

/** A saved scouting report (`library.Report`); its sections come from the scout endpoint. */
export type LibraryReport = {
  id: string;
  title: string;
  team_id: string;
  team_name: string;
  match_ids: string[];
  side: ReportSide;
  notes: string;
  created_at: string;
  updated_at: string;
};
