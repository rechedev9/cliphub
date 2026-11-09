import type { ScoutSkipReason } from './scout.ts';
import type { TacticalBuyType, TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of `GET /api/library/teams/{id}/grenades`
 * (`tacticalplan.TeamGrenades` in Go): every grenade a team threw on its matches.
 */

/** Grenade kinds in the order the orchestrator lists them (`tacticalplan.GrenadeKinds`). */
export const TEAM_GRENADE_KINDS = ['smoke', 'flash', 'he', 'molotov', 'decoy'] as const;
export type TeamGrenadeKind = (typeof TEAM_GRENADE_KINDS)[number];

/** What the throwers' team did in the 20 seconds after the grenade went off (`tacticalplan.GrenadeAfter`). */
export type GrenadeAfter = {
  /** Callout tokens a player of the team stayed in for 3 seconds, out of spawn. */
  zones: string[];
  /** Callout token of the team's player in its first duel of the window. */
  fight?: string;
};

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
  /** Callout tokens of the landing and of the origin; empty when none is near. */
  land_place: string;
  throw_place: string;
  /** Key of the repeated throw it belongs to, the same key the MCP's team_grenades names. */
  spot: string;
  after: GrenadeAfter;
};

/** One round the filter kept, seen from the throwers' side (`tacticalplan.GrenadeRound`). */
export type GrenadeRound = {
  match_id: string;
  round: number;
  /** Freeze end, where the round opens in the Replay. */
  tick: number;
  side: TacticalSide;
  buy: TacticalBuyType;
  opponent_buy: TacticalBuyType;
  won: boolean;
  /** The round's playbook category and site; empty when the round could not be read. */
  category: string;
  site: string;
  /** When the bomb went down, in seconds after freeze end. */
  plant_seconds: number | null;
};

export type TeamGrenades = {
  grenades: TeamGrenade[];
  /** The team's rounds the filter kept, with or without grenades. */
  rounds: number;
  round_list: GrenadeRound[];
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  awp_estimated?: number;
};

export const TEAM_GRENADES_KEYS = ['grenades', 'rounds', 'round_list', 'skipped', 'awp_estimated'] as const;
export const TEAM_GRENADES_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup', 'kind', 'rivals'] as const;
