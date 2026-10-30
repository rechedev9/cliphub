import type { ScoutEvidence } from './scout.ts';

/**
 * Browser-side contract of series, map history and the veto simulator
 * (`library.Series`, `library.MapRecord`, `library.VetoPlan`). Field names are
 * the Go JSON names; the Go package is the source of truth.
 */

export const SERIES_FORMATS = ['bo1', 'bo3', 'bo5'] as const;
export type SeriesFormat = (typeof SERIES_FORMATS)[number];

export const VETO_ACTIONS = ['ban', 'pick', 'decider'] as const;
export type VetoAction = (typeof VETO_ACTIONS)[number];

/** A series team slot: `a` and `b` are the two teams, `''` a decider. */
export type SeriesSlot = 'a' | 'b';
export type VetoTeam = SeriesSlot | '';

export type VetoStep = { team: VetoTeam; action: VetoAction; map: string };

export type SeriesTeam = { slot: SeriesSlot; team_id: string; name: string };

export type LibrarySeries = {
  id: string;
  format: SeriesFormat;
  event: string;
  veto?: string;
  team_ids: string[];
  teams: SeriesTeam[];
  vetoes: VetoStep[];
  match_ids: string[];
  created_at: string;
  updated_at: string;
};

export type SeriesInput = {
  format?: SeriesFormat;
  event?: string;
  veto?: string;
  team_ids?: string[];
  vetoes?: VetoStep[];
};

export type MapCount = { count: number; examples: ScoutEvidence[] };

export type SideRecord = {
  side: 'CT' | 'T';
  rounds: number;
  won: MapCount;
  lost: MapCount;
  pistol_won: MapCount;
  pistol_lost: MapCount;
};

export type VetoCount = { count: number; series_ids: string[] };

export type VetoRecord = {
  picked: VetoCount;
  banned: VetoCount;
  opponent_picked: VetoCount;
  opponent_banned: VetoCount;
  decider: VetoCount;
};

export type MapRecord = {
  map: string;
  matches: number;
  wins: number;
  losses: number;
  ties: number;
  rounds_won: number;
  rounds_lost: number;
  match_ids: string[];
  measured_matches: number;
  sides: SideRecord[];
  last_played_at?: string;
  veto: VetoRecord;
};

export type TeamMaps = {
  team_id: string;
  team_name: string;
  matches: number;
  series: number;
  skipped: { match_id: string; reason: string }[];
  maps: MapRecord[];
};

export type VetoBasis = 'chosen' | 'edge' | 'history' | 'leftover';

export type VetoMap = {
  map: string;
  our_matches: number;
  our_rounds_won: number;
  our_rounds: number;
  our_rate: number;
  their_matches: number;
  their_rounds_won: number;
  their_rounds: number;
  their_rate: number;
  edge: number;
  their_bans: number;
  their_picks: number;
};

export type VetoPlanStep = { index: number; team: VetoTeam; action: VetoAction; map: string; basis: VetoBasis };

export type VetoPlan = {
  format: SeriesFormat;
  first: SeriesSlot;
  pool: string[];
  model: { prior_rounds: number; history_min: number };
  maps: VetoMap[];
  steps: VetoPlanStep[];
  played: string[];
  opponent_history: number;
};

export type VetoSimulation = {
  team_id: string;
  team_name: string;
  opponent_id: string;
  opponent_name: string;
  plan: VetoPlan;
};

export type VetoQuery = {
  opponentId: string;
  format: SeriesFormat;
  first: SeriesSlot;
  pool: readonly string[];
  done: readonly string[];
};

/** The pool the simulator starts from (`library.DefaultMapPool`); the user edits it. */
export const DEFAULT_MAP_POOL = ['de_ancient', 'de_anubis', 'de_dust2', 'de_inferno', 'de_mirage', 'de_nuke', 'de_train'] as const;
/** Maps the pool editor offers besides the default pool. */
export const EXTRA_POOL_MAPS = ['de_overpass', 'de_vertigo'] as const;

export const SERIES_KEYS = ['id', 'format', 'event', 'veto', 'team_ids', 'teams', 'vetoes', 'match_ids', 'created_at', 'updated_at'];
export const SERIES_FIELDS = ['format', 'event', 'veto', 'team_ids', 'vetoes'];
export const TEAM_MAPS_KEYS = ['team_id', 'team_name', 'matches', 'series', 'skipped', 'maps'];
export const VETO_KEYS = ['team_id', 'team_name', 'opponent_id', 'opponent_name', 'plan'];
export const VETO_QUERY = ['opponent_id', 'format', 'first', 'pool', 'done'];

/** The query of a simulation, in a stable order. */
export function vetoSearch(query: VetoQuery): string {
  const params = new URLSearchParams({ opponent_id: query.opponentId, format: query.format, first: query.first });
  for (const map of query.pool) params.append('pool', map);
  for (const map of query.done) params.append('done', map);
  return params.toString();
}
