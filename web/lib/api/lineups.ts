import { TEAM_GRENADE_KINDS, type TeamGrenadeKind } from './grenades.ts';
import { jsonInit, readAll, request } from './library.ts';
import type { LibretaStatus } from './libreta.ts';
import type { ScoutEvidence } from './scout.ts';
import type { TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of the Libreta lineups (`library.Lineup` in Go): a
 * grenade saved with where to stand and aim, and how it is thrown.
 */

export const LINEUP_MOVEMENTS = ['standing', 'walking', 'running', 'crouching'] as const;
export type LineupMovement = (typeof LINEUP_MOVEMENTS)[number];

export const LINEUP_CLICKS = ['left', 'right', 'both'] as const;
export type LineupClick = (typeof LINEUP_CLICKS)[number];

export function isLineupMovement(value: string): value is LineupMovement {
  return LINEUP_MOVEMENTS.some((movement) => movement === value);
}

export function isLineupClick(value: string): value is LineupClick {
  return LINEUP_CLICKS.some((click) => click === value);
}

export function isLineupGrenade(value: string): value is TeamGrenadeKind {
  return TEAM_GRENADE_KINDS.some((kind) => kind === value);
}

export type LibraryLineup = {
  id: string;
  title: string;
  map: string;
  side: TacticalSide;
  grenade: TeamGrenadeKind;
  status: LibretaStatus;
  tags: string[];
  notes: string;
  /** Feet position, as `setpos` takes it. */
  origin: [number, number, number];
  /** Setang order: pitch from -90 (up) to 90 (down), yaw from -180 to 180. */
  pitch: number;
  yaw: number;
  landing: [number, number, number] | null;
  /** CS2 place tokens; empty when the map has no callout there. */
  throw_callout: string;
  landing_callout: string;
  movement: LineupMovement;
  jump: boolean;
  click: LineupClick;
  /** Movement and jump read from 8 Hz samples, until the user sets them. */
  technique_estimated: boolean;
  source: ScoutEvidence | null;
  /** The Lineups pro id this copy was saved from. */
  pro_lineup_id?: string;
  created_at: string;
  updated_at: string;
};

type LineupExtras = { status?: LibretaStatus; tags?: string[]; notes?: string };

/** The throw of a library round the server reads the lineup from. */
export type LineupThrow = { match_id: string; round: number; throw_tick: number };

export type LineupThrowInput = LineupExtras & { title: string; from: LineupThrow };

/** A lineup written by hand, usually from a pasted `setpos`. */
export type LineupInput = LineupExtras & {
  title: string;
  map: string;
  side: TacticalSide;
  grenade: TeamGrenadeKind;
  origin: [number, number, number];
  pitch: number;
  yaw: number;
  movement?: LineupMovement;
  jump?: boolean;
  click?: LineupClick;
  throw_callout?: string;
  landing_callout?: string;
};

/** Map, side and grenade are what a lineup is: a patch cannot change them. */
export type LineupPatch = Partial<
  Pick<
    LibraryLineup,
    'title' | 'status' | 'tags' | 'notes' | 'origin' | 'pitch' | 'yaw' | 'throw_callout' | 'landing_callout' | 'movement' | 'jump' | 'click'
  >
>;

export const LINEUP_KEYS = [
  'id', 'title', 'map', 'side', 'grenade', 'status', 'tags', 'notes', 'origin', 'pitch', 'yaw', 'landing',
  'throw_callout', 'landing_callout', 'movement', 'jump', 'click', 'technique_estimated', 'source', 'pro_lineup_id', 'created_at',
  'updated_at',
] as const satisfies readonly (keyof LibraryLineup)[];
export const LINEUP_LIST_QUERY = ['map', 'side', 'grenade', 'status', 'tag', 'include_archived'] as const;
export const LINEUP_PATCH_FIELDS = [
  'title', 'status', 'tags', 'notes', 'origin', 'pitch', 'yaw', 'throw_callout', 'landing_callout', 'movement', 'jump', 'click',
] as const satisfies readonly (keyof LineupPatch)[];
/** Fields a new lineup may send: a throw to read (`from`) or the lineup itself. */
export const LINEUP_FIELDS = ['from', 'map', 'side', 'grenade', ...LINEUP_PATCH_FIELDS] as const;

export type LineupListQuery = {
  map?: string;
  side?: TacticalSide;
  grenade?: TeamGrenadeKind;
  status?: LibretaStatus;
  tag?: string;
  /** Archived lineups are left out unless this is true or a status is asked for. */
  include_archived?: boolean;
};

/** The Libreta's lineups, by most recent edit. */
export function listLibraryLineups(query: LineupListQuery = {}): Promise<LibraryLineup[]> {
  const params = new URLSearchParams();
  if (query.map) params.set('map', query.map);
  if (query.side) params.set('side', query.side);
  if (query.grenade) params.set('grenade', query.grenade);
  if (query.status) params.set('status', query.status);
  if (query.tag) params.set('tag', query.tag);
  if (query.include_archived) params.set('include_archived', 'true');
  return readAll<LibraryLineup>('/lineups', 'lineups', params);
}

export function getLibraryLineup(lineupId: string): Promise<LibraryLineup> {
  return request<LibraryLineup>(`/lineups/${encodeURIComponent(lineupId)}`);
}

/** Saves a throw of a library round; the server reads its map, angles, landing and callouts. */
export function createLineupFromThrow(input: LineupThrowInput): Promise<LibraryLineup> {
  return request<LibraryLineup>('/lineups', jsonInit('POST', input));
}

export function createLibraryLineup(input: LineupInput): Promise<LibraryLineup> {
  return request<LibraryLineup>('/lineups', jsonInit('POST', input));
}

export function patchLibraryLineup(lineupId: string, patch: LineupPatch): Promise<LibraryLineup> {
  return request<LibraryLineup>(`/lineups/${encodeURIComponent(lineupId)}`, jsonInit('PATCH', patch));
}

export function deleteLibraryLineup(lineupId: string): Promise<void> {
  return request<void>(`/lineups/${encodeURIComponent(lineupId)}`, { method: 'DELETE' });
}
