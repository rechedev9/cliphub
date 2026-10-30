import type { ScoutEvidence } from './scout.ts';
import type { TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of the advanced metrics (`tacticalplan.MetricsReport`
 * plus the handler's scope and skip list). Field names are the Go JSON names.
 */

export type MetricCount = { count: number; examples: ScoutEvidence[] };

export type DuelBucket = { won: MetricCount; lost: MetricCount };

/** Duels split by who carried more round-start equipment. */
export type DuelMetrics = { even: DuelBucket; advantage: DuelBucket; disadvantage: DuelBucket };

export type EcoKill = ScoutEvidence & { value: number; killer_equip: number; victim_equip: number };

export type EcoMetrics = { kills: number; deaths: number; examples: EcoKill[] };

export const SWING_KINDS = ['kill', 'death', 'team_kill', 'plant', 'defuse'] as const;
export type SwingKind = (typeof SWING_KINDS)[number];

export type SwingEvent = ScoutEvidence & { kind: SwingKind; delta: number; before: number; after: number };

export type SwingMetrics = { wpa: number; per_round: number; best: SwingEvent[]; worst: SwingEvent[] };

export type TradeMetrics = { trade_kills: MetricCount; traded_deaths: MetricCount; untraded_deaths: MetricCount };

export type FlashMetrics = {
  thrown: number;
  enemies_blinded: MetricCount;
  team_blinded: MetricCount;
  enemy_blind_seconds: number;
  assists: MetricCount;
};

export type GrenadeDamage = { thrown: number; enemy_damage: number; team_damage: number; examples: ScoutEvidence[] };

export type UtilityMetrics = { measured: boolean; flash: FlashMetrics; he: GrenadeDamage; molotov: GrenadeDamage };

export type PositionLine = {
  side: TacticalSide;
  callout: string;
  rounds: number;
  won: number;
  kills: number;
  deaths: number;
  wpa: number;
  examples: ScoutEvidence[];
};

export type PlayerMatchLine = {
  match_id: string;
  map: string;
  played_at: string;
  team_key: string;
  rounds: number;
  kills: number;
  deaths: number;
  eco_kills: number;
  wpa: number;
};

export type PlayerMetrics = {
  steamid64: string;
  name: string;
  role: string;
  matches: number;
  rounds: number;
  kills: number;
  deaths: number;
  eco: EcoMetrics;
  duels: DuelMetrics;
  swing: SwingMetrics;
  trades: TradeMetrics;
  utility: UtilityMetrics;
  positions: PositionLine[];
  match_lines: PlayerMatchLine[];
};

export type RoleLine = {
  role: string;
  players: string[];
  rounds: number;
  kills: number;
  deaths: number;
  eco_kills: number;
  eco_deaths: number;
  wpa: number;
  flash_assists: number;
  utility_damage: number;
};

export type WinPoint = {
  tick: number;
  seconds: number;
  kind: string;
  actor_slot?: number;
  target_slot?: number;
  alive_ct: number;
  alive_t: number;
  probability_ct: number;
};

export type RoundTimeline = { match_id: string; round: number; winner: TacticalSide | ''; points: WinPoint[] };

export type WinModel = {
  version: string;
  alive_coef: number;
  equip_coef: number;
  clock_coef: number;
  plant_coef: number;
  bomb_coef: number;
  round_seconds: number;
  bomb_seconds: number;
  min_equip: number;
  map_priors: Record<string, number>;
};

export type MetricsThresholds = {
  duel_even_ratio: number;
  flash_min_seconds: number;
  trade_seconds: number;
  setup_seconds: number;
  example_limit: number;
};

export type MetricsSkipped = { match_id: string; reason: string };

export type MetricsReport = {
  match_id?: string;
  steamid64?: string;
  team_id?: string;
  team_name?: string;
  skipped: MetricsSkipped[];
  model: WinModel;
  thresholds: MetricsThresholds;
  matches: number;
  rounds: number;
  utility_matches: number;
  players: PlayerMetrics[];
  roles: RoleLine[];
  timelines?: RoundTimeline[];
};

/** Top-level keys the proxy copies from every metrics response. */
export const METRICS_KEYS = [
  'match_id', 'steamid64', 'team_id', 'team_name', 'skipped', 'model', 'thresholds', 'matches', 'rounds',
  'utility_matches', 'players', 'roles', 'timelines',
] as const;

/** Query names of a player's metrics. */
export const PLAYER_METRICS_QUERY = ['map', 'side', 'from', 'to', 'limit'] as const;
