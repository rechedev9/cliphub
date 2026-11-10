import type { ScoutSkipReason } from './scout.ts';
import type { TacticalBuyType, TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of `GET /api/library/teams/{id}/grenades`
 * (`tacticalplan.TeamGrenades` in Go): every grenade a team threw on its matches.
 */

/** Grenade kinds in the order the orchestrator lists them (`tacticalplan.GrenadeKinds`). */
export const TEAM_GRENADE_KINDS = ['smoke', 'flash', 'he', 'molotov', 'decoy'] as const;
export type TeamGrenadeKind = (typeof TEAM_GRENADE_KINDS)[number];

export type TeamGrenade = {
  match_id: string;
  round: number;
  /** When the grenade went off; `seconds` is that tick after freeze end. */
  tick: number;
  seconds: number;
  kind: TeamGrenadeKind;
  side: TacticalSide;
  thrower_steamid64: string;
  thrower_name: string;
  /** Where the thrower stood; null on a match analysed before throws were recorded. */
  origin: [number, number, number] | null;
  throw_tick: number | null;
  /** Eye yaw and pitch in degrees at the throw. */
  view: [number, number] | null;
  landing: [number, number, number];
  buy: TacticalBuyType;
  round_won: boolean;
};

export type TeamGrenades = {
  grenades: TeamGrenade[];
  /** The team's rounds the filter kept, with or without grenades. */
  rounds: number;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  awp_estimated?: number;
};

export const TEAM_GRENADES_KEYS = ['grenades', 'rounds', 'skipped', 'awp_estimated'] as const;
export const TEAM_GRENADES_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup', 'kind'] as const;
