import { request, teamSpatialParams, type TeamSpatialQuery } from './library.ts';
import type { ScoutEvidence, ScoutSkipReason } from './scout.ts';
import type { TacticalSide } from './tactical.ts';

/** Browser-side contract of `tacticalplan.FirstDeathPositions` and `tacticalplan.PrepWarning`; Go is the source of truth. */

export const FIRST_DEATH_MOVEMENTS = ['static', 'slow', 'fast'] as const;
export type FirstDeathMovement = (typeof FIRST_DEATH_MOVEMENTS)[number];

/** A callout where the player stood: `seconds` per round played, `samples` world positions to draw. */
export type FirstDeathZone = {
  callout: string;
  seconds: number;
  rounds: number;
  refs: ScoutEvidence[];
  samples: [number, number, number][];
  floor?: 'lower';
};

export type FirstDeathPlayer = { steamid64: string; name: string; role: string; rounds: number; zones: FirstDeathZone[] };

export type TeamFirstDeath = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  map: string;
  side: TacticalSide;
  window: { from_seconds: number; to_seconds: number };
  movement: FirstDeathMovement;
  thresholds: { static_below_speed: number; fast_above_speed: number; zone_limit: number; sample_limit: number };
  matches: number;
  round_count: number;
  framed: number;
  players: FirstDeathPlayer[];
};

export const FIRST_DEATH_KEYS = [
  'team_id', 'team_name', 'skipped', 'map', 'side', 'window', 'movement', 'thresholds', 'matches', 'round_count', 'framed', 'players',
] as const satisfies readonly (keyof TeamFirstDeath)[];
export const FIRST_DEATH_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup', 't_from', 't_to', 'movement'] as const;

/** The team scope plus the window after freeze end (`to` 0 runs to the round end) and the movement. */
export type FirstDeathQuery = TeamSpatialQuery & { map: string; side: TacticalSide; from_seconds: number; to_seconds: number; movement: FirstDeathMovement };

export function getTeamFirstDeath(teamId: string, query: FirstDeathQuery): Promise<TeamFirstDeath> {
  const params = teamSpatialParams(query);
  params.set('t_from', String(query.from_seconds));
  if (query.to_seconds > 0) params.set('t_to', String(query.to_seconds));
  params.set('movement', query.movement);
  return request<TeamFirstDeath>(`/teams/${encodeURIComponent(teamId)}/first-death?${params.toString()}`);
}

/** One reason to distrust a preparation, as the server found it over the kept matches. */
export type PrepWarning =
  | { kind: 'stale'; days: number; last_played_at?: string }
  | { kind: 'lineup'; changed: number; matches: number }
  | { kind: 'small_sample'; rounds: number };
