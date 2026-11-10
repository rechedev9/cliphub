import type { LibraryRosterEntry } from './library.ts';
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
  /** Every hit round; `examples` holds the first ones. */
  refs?: ScoutEvidence[];
};

/** Attacking one site: `attacks` is over the T rounds of the map, `post_plant_won` over the plants. */
export type BombsiteAttack = {
  attacks: TacticalRate;
  plants: TacticalRate;
  won: TacticalRate;
  post_plant_won: TacticalRate;
  examples: ScoutEvidence[];
  /** Every attacked round; `examples` holds the first ones. */
  refs?: ScoutEvidence[];
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

/** What the lineup picker chooses from: who played the team's latest match, then everyone else. */
export type TeamLineups = {
  team_id: string;
  team_name: string;
  latest_match: { match_id: string; map: string; played_at: string } | null;
  latest: LibraryRosterEntry[];
  others: LibraryRosterEntry[];
};

/** Top-level keys the proxy copies, and the query names it forwards. */
export const TEAM_BOMBSITES_KEYS = ['team_id', 'team_name', 'skipped', 'filter', 'min_reliable_sample', 'match_count', 'maps'] as const;
export const TEAM_BOMBSITES_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup'] as const;
export const PLAYER_POSITIONS_KEYS = [
  'steamid64', 'skipped', 'match_count', 'window_seconds', 'place_limit', 'anchor_share', 'min_reliable_sample', 'maps',
] as const;
export const PLAYER_POSITIONS_QUERY = ['map', 'from', 'to'] as const;
export const TEAM_LINEUPS_KEYS = ['team_id', 'team_name', 'latest_match', 'latest', 'others'] as const;

/** The team scope keys every team endpoint takes beyond the scout filter (`httpapi.teamScopeQuery`). */
export const TEAM_SCOPE_QUERY = [
  'max_matches', 'opponent_top', 'whose', 'buy', 'opponent_buy', 'awp', 'opponent_awp', 'advantage', 'round_from', 'round_to',
] as const;

/** Mi equipo and the HLTV rank of a team (`library.TeamProfile`); `own_teams` counts the teams marked as own. */
export type TeamProfile = { team_id: string; own: boolean; hltv_rank: number; updated_at?: string; own_teams: number };
export const TEAM_PROFILE_KEYS = ['team_id', 'own', 'hltv_rank', 'updated_at', 'own_teams'] as const;
export const TEAM_PROFILE_FIELDS = ['own', 'hltv_rank'] as const;

/** One kept round of a team's rounds list (`httpapi.teamRoundOut`). */
export type TeamRound = { round: number; side: TacticalSide; winner: TacticalSide | ''; buy: string; opponent_buy: string; site: string };

export type TeamRoundsMatch = {
  match_id: string;
  map: string;
  played_at: string;
  team_key: string;
  opponent?: string;
  rounds: TeamRound[];
  /** Rounds the match has before the filter. */
  total: number;
};

/** The rounds the team scope keeps per match, newest match first. */
export type TeamRounds = {
  team_id: string;
  team_name: string;
  matches: TeamRoundsMatch[];
  rounds: number;
  skipped: Skipped[];
  /** Matches whose AWP filter read kills and hits: their analysis predates the AWP flag. */
  awp_estimated?: number;
};
export const TEAM_ROUNDS_KEYS = ['team_id', 'team_name', 'matches', 'rounds', 'skipped', 'awp_estimated'] as const;

/** The skip reason of a match the opponent level could not rate. */
export const OPPONENT_UNRANKED = 'opponent_unranked';

/** The skip reason of a match past the server's cap of 50 (`library.MaxReportMatches`). */
export const OVER_MATCH_LIMIT = 'over_match_limit';

/** What the sample box adds to a team figure's count: unrated rivals, the 50 match cap and estimated AWP. */
export type SampleNotes = { unrated: number; capped: number; awpEstimated: number };

export function sampleNotes(skipped: readonly Skipped[], awpEstimated: number | undefined): SampleNotes {
  return {
    unrated: skipped.filter((entry) => entry.reason === OPPONENT_UNRANKED).length,
    capped: skipped.filter((entry) => entry.reason === OVER_MATCH_LIMIT).length,
    awpEstimated: awpEstimated ?? 0,
  };
}
