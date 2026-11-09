import type { PrepRounds, PrepSpot, PrepWindow } from './playbook.ts';
import type { ScoutSkipReason } from './scout.ts';
import type { TacticalRate } from './tactical.ts';

/** Browser-side contract of the T side match prep (`tacticalplan.AttackPrep`). Field names are the Go JSON names. */

export const ATTACK_SITES = ['A', 'B', 'both', 'none'] as const;
export type AttackSite = (typeof ATTACK_SITES)[number];
export const ATTACK_TEMPOS = ['rush', 'set_strat', 'mid_round', 'no_site'] as const;
export type AttackTempo = (typeof ATTACK_TEMPOS)[number];

/** Buys in bands: semi and force buys count as half. */
export const ATTACK_BANDS = ['pistol', 'eco', 'half', 'full'] as const;
export type AttackBand = (typeof ATTACK_BANDS)[number];

export type AttackCell = PrepRounds & { key: string };

export type AttackMatchup = PrepRounds & { buy: AttackBand; opponent_buy: AttackBand; sites: AttackCell[]; tempo: AttackCell[] };

export type AttackZone = PrepRounds & { zone: string; label: string };

/** `formation` counts the players leaning A, mid and B at the read second, such as `2-1-2`. */
export type AttackDefault = PrepRounds & { formation: string; zones: AttackZone[] };

export type AttackExecute = PrepRounds & {
  buy: AttackBand;
  opponent_buy: AttackBand;
  site: AttackSite;
  /** The callout the players stepped onto the site from; empty when no route repeats. */
  route: string;
  route_label: string;
  rushes: TacticalRate;
  plants: TacticalRate;
  contact_seconds: number;
};

export type AttackSpot = PrepSpot & { arrival_seconds: number };

export type AttackPlayerWindow = PrepWindow & {
  rounds: number;
  spots: AttackSpot[];
  pace: { static: TacticalRate; slow: TacticalRate; fast: TacticalRate };
};

export type AttackPlayer = { steamid64: string; name: string; role: string; rounds: number; windows: AttackPlayerWindow[] };

export type AttackWarning = { kind: 'stale' | 'few_rounds' | 'roster_changed'; days?: number; rounds?: number; joined?: string[]; left?: string[] };

export type TeamAttack = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  map: string;
  matches: number;
  round_count: number;
  framed: number;
  last_played_at?: string;
  days_since_last_match: number;
  sites: AttackCell[];
  tempo: AttackCell[];
  matchups: AttackMatchup[];
  defaults: AttackDefault[];
  executes: AttackExecute[];
  players: AttackPlayer[];
  warnings: AttackWarning[];
};

export const ATTACK_QUERY = ['map', 'from', 'to', 'opponent_id', 'match_id', 'lineup'] as const;
export const ATTACK_KEYS = [
  'team_id', 'team_name', 'skipped', 'map', 'thresholds', 'matches', 'round_count', 'framed', 'last_played_at',
  'days_since_last_match', 'sites', 'tempo', 'matchups', 'defaults', 'executes', 'players', 'warnings',
] as const;
