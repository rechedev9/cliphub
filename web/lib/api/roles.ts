import { jsonInit, request, type LibraryRole } from './library.ts';
import type { ScoutSkipReason } from './scout.ts';

/**
 * Browser-side contract of the roles (`library.RoleSlot`, `library.TeamRoleSheet`
 * and the suggestion of GET /teams/{id}/roles). Field names are the Go JSON names.
 */

/** A side has five players (`library.MaxRoleSlots`). */
export const MAX_ROLE_SLOTS = 5;
export const MAX_ROLE_NAME = 40;
export const MAX_ROLE_TASK = 200;
export const MAX_ROLE_CALLOUT = 64;

/** One player's job in a play or on a team's role sheet. */
export type RoleSlot = {
  name: string;
  steamid64?: string;
  /** Where they play: a CS2 place token or the coach's own words. */
  callout?: string;
  task?: string;
};

/** T roles are the inferred player roles; CT ones come from where each player sets up. */
export type SuggestedRole = LibraryRole | 'anchor' | 'rotator';

export type RoleSuggestion = {
  steamid64: string;
  player: string;
  role: SuggestedRole;
  /** CT only: the CS2 place token where the player sets up most. */
  callout?: string;
  rounds: number;
};

export type TeamRoleSheet = {
  team_id: string;
  map: string;
  t: RoleSlot[];
  ct: RoleSlot[];
  /** Missing until the sheet is saved once. */
  updated_at?: string;
};

export type TeamRoles = {
  team_id: string;
  team_name: string;
  sheet: TeamRoleSheet;
  suggested: { t: RoleSuggestion[]; ct: RoleSuggestion[] };
  matches: number;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
};

export type RoleSheetInput = { map: string; t: RoleSlot[]; ct: RoleSlot[] };

export const TEAM_ROLES_KEYS = ['team_id', 'team_name', 'sheet', 'suggested', 'matches', 'skipped'] as const;
export const ROLE_SHEET_KEYS = ['team_id', 'map', 't', 'ct', 'updated_at'] as const;
export const ROLE_SHEET_FIELDS = ['map', 't', 'ct'] as const satisfies readonly (keyof RoleSheetInput)[];

/** The team's saved sheet for one map and a suggestion per side read from its matches there. */
export function getTeamRoles(teamId: string, map: string): Promise<TeamRoles> {
  const params = new URLSearchParams({ map });
  return request<TeamRoles>(`/teams/${encodeURIComponent(teamId)}/roles?${params.toString()}`);
}

/** Replaces the team's whole sheet of one map, both sides. */
export function setTeamRoles(teamId: string, sheet: RoleSheetInput): Promise<TeamRoleSheet> {
  return request<TeamRoleSheet>(`/teams/${encodeURIComponent(teamId)}/roles`, jsonInit('PUT', sheet));
}
