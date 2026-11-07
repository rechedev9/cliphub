import type { CalloutName } from './callouts.ts';
import type { LibraryCall } from './calls.ts';
import type { LibretaStatus } from './libreta.ts';
import type { LibraryLineup } from './lineups.ts';
import type { RoleSlot } from './roles.ts';
import type { ScoutEvidence, ScoutSkipReason } from './scout.ts';
import type { TacticalBuyType, TacticalEventKind, TacticalRate, TacticalSide, TacticalSite } from './tactical.ts';

/**
 * Browser-side contract of the stratbook (`library.Play`, `tacticalplan.PlayDetection`
 * and the lineup comparison of a round). Field names are the Go JSON names.
 */

/** The grenades a play can hold: the utility the round classifier counts. */
export const PLAY_GRENADES = ['smoke', 'flash', 'molotov', 'he'] as const;
export type PlayGrenadeKind = (typeof PLAY_GRENADES)[number];

export type PlayOrigin = 'round' | 'detected' | 'import' | 'manual' | 'playbook';

/** One grenade of a play: when it landed after freeze end and where (world units). */
export type PlayGrenade = {
  kind: PlayGrenadeKind;
  seconds: number;
  pos: [number, number, number];
  callout?: string;
  player?: string;
  /** On a detected play: the part of its rounds that threw it, 0..1. */
  share?: number;
  lineup_id?: string;
  lineup?: string;
};

/** Where one player of a play stood at the play's second. */
export type PlayPosition = {
  player?: string;
  steamid64?: string;
  pos: [number, number, number];
  callout?: string;
  share?: number;
};

export type LibraryPlay = {
  id: string;
  title: string;
  team_id: string | null;
  team_name?: string;
  map: string;
  side: TacticalSide;
  /** A round pattern of the side (`TacticalTPattern` or `TacticalCTPattern`). */
  kind: string;
  site: TacticalSite;
  origin: PlayOrigin;
  positions_seconds: number;
  grenades: PlayGrenade[];
  positions: PlayPosition[];
  notes: string;
  examples: ScoutEvidence[];
  status: LibretaStatus;
  tags: string[];
  /** Who does what in the play, at most `MAX_ROLE_SLOTS`. */
  roles: RoleSlot[];
  created_at: string;
  updated_at: string;
};

/** One play as GET /plays/{id} returns it: with the example matches not in this library. */
export type LibraryPlayDetail = LibraryPlay & { missing_match_ids: string[] };

/** What a new play sends; the server fills the id, the team name and the dates. */
export type PlayInput = Omit<LibraryPlay, 'id' | 'team_name' | 'created_at' | 'updated_at' | 'status' | 'tags' | 'roles'> &
  Partial<Pick<LibraryPlay, 'status' | 'tags' | 'roles'>>;

export type PlayPatch = Partial<
  Pick<LibraryPlay, 'title' | 'team_id' | 'kind' | 'site' | 'notes' | 'grenades' | 'positions' | 'examples' | 'status' | 'tags' | 'roles'>
>;

export type PlayThresholds = {
  position_seconds: number;
  min_grenades: number;
  min_rounds: number;
  min_utility_similarity: number;
  min_position_similarity: number;
  common_share: number;
  example_limit: number;
  skipped_buys: TacticalBuyType[];
};

/** A group of a team's rounds that share their utility and positions. */
export type DetectedPlay = {
  map: string;
  side: TacticalSide;
  kind: string;
  site: TacticalSite;
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
  grenades: PlayGrenade[];
  positions: PlayPosition[];
  seed: ScoutEvidence;
  examples: ScoutEvidence[];
};

export type PlayDetection = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  thresholds: PlayThresholds;
  matches: number;
  rounds: number;
  compared: number;
  plays: DetectedPlay[];
};

export type LineupStatus = 'matched' | 'unmatched' | 'no_catalog' | 'not_covered';

export type LineupMatch = {
  id?: string;
  destination?: string;
  from_area?: string;
  side?: string;
  confidence?: number;
  distance_units?: number;
};

export type RoundLineupGrenade = {
  tick: number;
  seconds: number;
  kind: TacticalEventKind;
  side?: TacticalSide;
  player?: string;
  steamid64?: string;
  pos: [number, number, number];
  callout?: string;
  status: LineupStatus;
  lineup?: LineupMatch;
};

export type RoundLineups = {
  match_id: string;
  round: number;
  map: string;
  catalog_size: number;
  throw_checked: boolean;
  smokes: number;
  matched: number;
  grenades: RoundLineupGrenade[];
};

/** The exported stratbook (`stratbookFile` in the orchestrator). */
export const STRATBOOK_FORMAT = 'cliphub-stratbook';
/** The version Studio writes; it still reads every older one. */
export const STRATBOOK_VERSION = 2;

/** The sections besides plays that the whole Libreta export writes, each left out when empty. */
export const LIBRETA_SECTIONS = ['lineups', 'team_roles', 'calls', 'callout_names'] as const;
export type LibretaSection = (typeof LIBRETA_SECTIONS)[number];

/** A role sheet in a file, with its team by name since team ids mean nothing on another PC. */
export type FileTeamRoles = { team_name: string; map: string; t: RoleSlot[]; ct: RoleSlot[] };

export type StratbookFile = {
  format: typeof STRATBOOK_FORMAT;
  version: number;
  exported_at: string;
  plays: LibraryPlay[];
  lineups?: LibraryLineup[];
  team_roles?: FileTeamRoles[];
  calls?: LibraryCall[];
  callout_names?: CalloutName[];
};

/** A file the user picked: the orchestrator checks every record of every section, one by one. */
export type StratbookUpload = Pick<StratbookFile, 'format' | 'version' | 'exported_at'> & {
  plays: unknown[];
} & Partial<Record<LibretaSection, unknown[]>>;

/** What one section of an imported file did. */
export type SectionImport = { imported: number; duplicates: number; rejected: number };

export type StratbookImport = {
  imported: number;
  duplicates: number;
  rejected: { index: number; title: string; error: string }[];
  plays: LibraryPlay[];
  /** Only the sections the file held. */
  sections?: Partial<Record<LibretaSection, SectionImport>>;
};

/** Response keys the library proxy lets through, per route. */
export const PLAY_KEYS = [
  'id', 'title', 'team_id', 'team_name', 'map', 'side', 'kind', 'site', 'origin', 'positions_seconds',
  'grenades', 'positions', 'notes', 'examples', 'status', 'tags', 'roles', 'created_at', 'updated_at',
] as const;
export const PLAY_DETAIL_KEYS = [...PLAY_KEYS, 'missing_match_ids'] as const;
/** Fields a new play may send. */
export const PLAY_CREATE_FIELDS = [
  'title', 'team_id', 'map', 'side', 'kind', 'site', 'origin', 'positions_seconds', 'grenades', 'positions', 'notes', 'examples',
  'status', 'tags', 'roles',
] as const;
/** Map, side and origin are what a play is: a patch cannot change them. */
export const PLAY_PATCH_FIELDS = [
  'title', 'team_id', 'kind', 'site', 'notes', 'grenades', 'positions', 'examples', 'status', 'tags', 'roles',
] as const;
export const PLAY_DETECTION_KEYS = [
  'team_id', 'team_name', 'skipped', 'thresholds', 'matches', 'rounds', 'compared', 'plays',
] as const;
export const ROUND_LINEUPS_KEYS = [
  'match_id', 'round', 'map', 'catalog_size', 'throw_checked', 'smokes', 'matched', 'grenades',
] as const;
export const STRATBOOK_FILE_KEYS = ['format', 'version', 'exported_at', 'plays', ...LIBRETA_SECTIONS] as const;
export const STRATBOOK_IMPORT_KEYS = ['imported', 'duplicates', 'rejected', 'plays', 'sections'] as const;
export const PLAY_LIST_QUERY = ['team_id', 'map', 'side', 'kind', 'status', 'tag', 'include_archived'] as const;
