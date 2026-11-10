import { jsonInit, request, teamSpatialParams } from './library.ts';
import type { TeamSpatialQuery } from './library.ts';
import type { SpatialResponse } from './spatial.ts';
import { SPATIAL_RESPONSE_KEYS } from './spatial.ts';
import type { TacticalBuyType, TacticalSide } from './tactical.ts';
import type { TeamGrenadeKind } from './grenades.ts';
import { TEAM_SCOPE_QUERY } from './team-insights.ts';

// The pattern finder's contract (`tacticalplan.Pattern`, `library.Pattern`): blocks that must all
// hold in a round, run over a team's matches or saved and detected over the whole library.

export const PATTERN_BLOCK_KINDS = ['player', 'utility', 'plant'] as const;
export type PatternBlockKind = (typeof PATTERN_BLOCK_KINDS)[number];

/** The server's caps (`tacticalplan.PatternMaxBlocks`, `library.MaxPatternNameRunes`, `library.MaxPatternMatches`). */
export const PATTERN_MAX_BLOCKS = 6;
export const PATTERN_MAX_PLAYERS = 10;
export const PATTERN_NAME_MAX = 80;
export const PATTERN_MATCHES_MAX = 50;

/** A box in world units; `level` keeps it to one floor of a two-floor map. */
export type PatternArea = { min_x: number; min_y: number; max_x: number; max_y: number; level?: 'default' | 'lower' };

/** One condition. `from` and `to` are seconds after freeze end; `to` 0 or absent runs to the end of the round. */
export type PatternBlock = {
  kind: PatternBlockKind;
  /** SteamID64s; empty is anyone of the side. */
  players?: string[];
  /** Reads the rival team's players instead of the team's. */
  rival?: boolean;
  grenade?: TeamGrenadeKind;
  /** Places a grenade where it was thrown instead of where it landed. */
  from_throw?: boolean;
  area?: PatternArea;
  from: number;
  to?: number;
  /** Keeps the rounds in which the block does not hold. */
  invert?: boolean;
};

export type PatternDefinition = { side?: TacticalSide; blocks: PatternBlock[] };

export type PatternHit = { matched: boolean; seconds?: number };

export type PatternRound = {
  round: number;
  side: TacticalSide;
  won: boolean;
  buy?: TacticalBuyType;
  opponent_buy?: TacticalBuyType;
  hits: PatternHit[];
  /** When the first timed block first held, and its tick. */
  seconds?: number;
  tick?: number;
  /** Seconds from freeze end to the plant, when the bomb went down. */
  plant_seconds?: number;
};

export type PatternMatch = {
  match_id: string;
  played_at: string;
  team_key?: string;
  team?: string;
  opponent?: string;
  rounds: number;
  matched: PatternRound[];
};

export type PatternTrend = 'up' | 'down' | 'flat' | '';

export type PatternResult = {
  rounds: number;
  matched: number;
  won: number;
  /** 0 to 1 over the matched rounds. */
  win_rate: number;
  blocks: { matched: number; rounds: number }[];
  /** Matches without saved positions: a player block leaves their rounds out of scope. */
  missing_positions?: number;
  unreadable_matches?: number;
  /** Grenades a throw-position block could not place. */
  grenades_without_throw?: number;
  /** A library-wide run read only this many of the newest matches of the map. */
  capped_at?: number;
  recent_rounds: number;
  recent_matched: number;
  trend: PatternTrend;
  months: { month: string; rounds: number; matched: number }[];
  matches: PatternMatch[];
};

export type PatternRunResponse = SpatialResponse & { pattern: PatternDefinition; result: PatternResult };

/** A saved pattern's own scope; `half` is a semi or a force buy, as the scope bar holds it. */
export type PatternScopeBuy = TacticalBuyType | 'half';
export type PatternScope = { matches?: number; buy?: PatternScopeBuy; opponent_buy?: PatternScopeBuy };

export type PatternStats = { rounds: number; matched: number; won: number; win_rate: number; recent_matched: number; trend: PatternTrend };

export type SavedPattern = {
  id: string;
  name: string;
  map: string;
  /** Without a team the pattern reads the whole library. */
  team_id?: string;
  pattern: PatternDefinition;
  scope: PatternScope;
  created_at: string;
  updated_at: string;
  stats?: PatternStats;
};

export type PatternInput = { name: string; map: string; team_id?: string; pattern: PatternDefinition; scope?: PatternScope };

export const PATTERN_RUN_KEYS = [...SPATIAL_RESPONSE_KEYS, 'pattern', 'result'] as const;
export const SAVED_PATTERN_KEYS = [
  'id', 'name', 'map', 'team_id', 'pattern', 'scope', 'created_at', 'updated_at',
] as const satisfies readonly (keyof SavedPattern)[];
export const PATTERN_FIELDS = ['name', 'map', 'team_id', 'clear_team', 'pattern', 'scope'] as const;
/** The team view's scope, its N most recent matches and the pattern as JSON. */
export const TEAM_PATTERN_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup', ...TEAM_SCOPE_QUERY, 'pattern'] as const;
export const PATTERN_LIST_QUERY = ['map', 'team_id', 'stats', 'side', 'from', 'to', 'opponent_id', 'lineup', ...TEAM_SCOPE_QUERY] as const;

/** The team scope a run reads; the bar's other filters, the last N matches among them, travel as `criteria`. */
export type TeamPatternScope = TeamSpatialQuery;

function scopeParams(scope: TeamPatternScope): URLSearchParams {
  return teamSpatialParams(scope);
}

export type TeamPatternRun = { teamId: string; scope: TeamPatternScope; pattern: PatternDefinition };

/** A pattern over a team's matches on one map: the matched rounds by match, their paths and heatmap. */
export function runTeamPattern(run: TeamPatternRun): Promise<PatternRunResponse> {
  const params = scopeParams(run.scope);
  params.set('pattern', JSON.stringify(run.pattern));
  return request<PatternRunResponse>(`/teams/${encodeURIComponent(run.teamId)}/pattern?${params.toString()}`);
}

/** A saved pattern's own rounds: over its team, or over the library when it has none. */
export function savedPatternRounds(id: string): Promise<PatternRunResponse> {
  return request<PatternRunResponse>(`/patterns/${encodeURIComponent(id)}/rounds`);
}

/** The saved patterns of a map, the team's and the library-wide ones; figures use each one's scope. */
export async function listSavedPatterns(query: { map: string; teamId?: string; scope?: TeamPatternScope; stats?: boolean }): Promise<SavedPattern[]> {
  const params = query.scope === undefined ? new URLSearchParams() : scopeParams(query.scope);
  params.set('map', query.map);
  if (query.teamId !== undefined) params.set('team_id', query.teamId);
  if (query.stats === true) params.set('stats', '1');
  const body = await request<{ patterns?: SavedPattern[] }>(`/patterns?${params.toString()}`);
  return body.patterns ?? [];
}

export function createSavedPattern(input: PatternInput): Promise<SavedPattern> {
  return request<SavedPattern>('/patterns', jsonInit('POST', input));
}

export function deleteSavedPattern(id: string): Promise<void> {
  return request<void>(`/patterns/${encodeURIComponent(id)}`, jsonInit('DELETE'));
}
