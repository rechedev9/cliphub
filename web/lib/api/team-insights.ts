import type { ScoutEvidence, ScoutFilter } from './scout.ts';
import type { TacticalRate, TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of a team's bombsite record (`tacticalplan.BombsiteReport`)
 * and a player's early-round positions (`tacticalplan.PlayerPositionsReport`).
 * Field names are the Go JSON names; the Go package is the source of truth.
 */

type Skipped = { match_id: string; reason: string };

/** Defending one site: `hit` is over the CT rounds of the map, the rest over the hits, plants or attempts. */
export type BombsiteDefense = {
  hit: TacticalRate;
  held: TacticalRate;
  plants_allowed: TacticalRate;
  won: TacticalRate;
  retakes_attempted: TacticalRate;
  retakes_won: TacticalRate;
  examples: ScoutEvidence[];
};

/** Attacking one site: `attacks` is over the T rounds of the map, `post_plant_won` over the plants. */
export type BombsiteAttack = {
  attacks: TacticalRate;
  plants: TacticalRate;
  won: TacticalRate;
  post_plant_won: TacticalRate;
  examples: ScoutEvidence[];
};

export type BombsiteSite = { site: 'A' | 'B'; ct: BombsiteDefense; t: BombsiteAttack };

export type BombsiteMap = { map: string; matches: number; ct_rounds: number; t_rounds: number; sites: BombsiteSite[] };

export type TeamBombsites = {
  team_id: string;
  team_name: string;
  skipped: Skipped[];
  filter: ScoutFilter;
  min_reliable_sample: number;
  match_count: number;
  maps: BombsiteMap[];
};

/** A CT player's site role; empty on T. */
export type PositionRole = 'a_anchor' | 'b_anchor' | 'rotator';
export type PositionZone = 'A' | 'B' | 'other';

/** A callout the player held most of a round's opening window: `place` is the CS2 token, `label` the callout. */
export type PositionPlace = { place: string; label: string; rounds: TacticalRate; examples: ScoutEvidence[] };

export type PlayerSidePositions = {
  side: TacticalSide;
  rounds: number;
  /** Rounds with a position read; zone and place rates are over it. */
  sampled: number;
  role?: PositionRole;
  zones: { zone: PositionZone; rounds: TacticalRate }[];
  places: PositionPlace[];
  awp_kills: TacticalRate;
  awp_rounds: TacticalRate;
};

export type PlayerMapPositions = { map: string; matches: number; sides: PlayerSidePositions[] };

export type PlayerPositions = {
  steamid64: string;
  skipped: Skipped[];
  match_count: number;
  window_seconds: number;
  place_limit: number;
  anchor_share: number;
  min_reliable_sample: number;
  maps: PlayerMapPositions[];
};

export type PlayerPositionsQuery = { map?: string; from?: string; to?: string };

/** Top-level keys the proxy copies, and the query names it forwards. */
export const TEAM_BOMBSITES_KEYS = ['team_id', 'team_name', 'skipped', 'filter', 'min_reliable_sample', 'match_count', 'maps'] as const;
export const TEAM_BOMBSITES_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id'] as const;
export const PLAYER_POSITIONS_KEYS = [
  'steamid64', 'skipped', 'match_count', 'window_seconds', 'place_limit', 'anchor_share', 'min_reliable_sample', 'maps',
] as const;
export const PLAYER_POSITIONS_QUERY = ['map', 'from', 'to'] as const;
