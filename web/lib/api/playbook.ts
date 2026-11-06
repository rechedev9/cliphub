import type { ScoutEvidence, ScoutRounds, ScoutSkipReason, ScoutSpot } from './scout.ts';
import type { TacticalBuyType, TacticalHistogram, TacticalRate, TacticalSide, TacticalSite } from './tactical.ts';

/**
 * Browser-side contract of the team Playbook (`tacticalplan.Playbook`) and the
 * Match Prep (`tacticalplan.Prep`). Field names are the Go JSON names.
 */

/** Each side's categories in the order the server tries them; a round gets the first that fits. */
export const PLAYBOOK_CATEGORIES = {
  T: ['save', 'fake', 'rush', 'execute', 'map_control', 'default'],
  CT: ['save', 'stack', 'aggression', 'retake', 'default'],
} as const;
export type PlaybookCategoryName = (typeof PLAYBOOK_CATEGORIES)[keyof typeof PLAYBOOK_CATEGORIES][number];

/** A set of rounds with the ref of every one, to open each in the replay. */
export type PlaybookGroup = {
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
  refs: ScoutEvidence[];
};

export type PlaybookCategory = PlaybookGroup & { category: PlaybookCategoryName };
export type PlaybookSite = PlaybookGroup & { site: TacticalSite };
/** `share` is over the rounds with positions, the only ones a hold is read from. */
export type PlaybookZone = PlaybookGroup & { zone: string; label: string };

/** A zone the team held: 2+ players together for 5+ unbroken seconds before 40 s. */
export type ZoneHold = { zone: string; from_seconds: number; seconds: number; players: number };

export type PlaybookRound = {
  match_id: string;
  round: number;
  /** Freeze end, the tick to open the replay at. */
  tick: number;
  category: PlaybookCategoryName;
  /** The round classifier's pattern for the side. */
  pattern: string;
  site: TacticalSite;
  fake_site?: TacticalSite;
  won: boolean;
  buy: TacticalBuyType;
  opponent_buy: TacticalBuyType;
  /** Seconds to the site contact or plant (T) or the first duel (CT); null when it never came. */
  tempo_seconds: number | null;
  framed: boolean;
  zones: ZoneHold[];
  /** Every tactic key the round shows, such as `zone:Middle`. */
  tactics: string[];
};

/** Tactic kinds in display order (`tacticalplan.TacticKinds`). */
export const TACTIC_KINDS = ['category', 'site', 'buy', 'opponent_buy', 'opening', 'opening_result', 'awp', 'zone', 'utility'] as const;
export type TacticKind = (typeof TACTIC_KINDS)[number];

/** The rounds that show one tactic; `share` is over the rounds the combination kept. */
export type PlaybookTactic = PlaybookGroup & {
  key: string;
  kind: TacticKind;
  /** A category, site, buy, callout, `won` or `lost`. */
  value: string;
  grenade?: 'smoke' | 'molotov';
  /** The callout's name on tactics read at a place. */
  label?: string;
};

/** A round stays when it shows every `with` tactic and no `without` tactic. */
export type PlaybookCombo = { with: string[]; without: string[] };

export type PlaybookThresholds = {
  rush_seconds: number;
  fake_utility: number;
  site_contact_players: number;
  map_control_players: number;
  map_control_seconds: number;
  map_control_window_seconds: number;
  tempo_bucket_seconds: number;
};

export type TeamPlaybook = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  map: string;
  side: TacticalSide;
  thresholds: PlaybookThresholds;
  matches: number;
  round_count: number;
  win_rate: TacticalRate;
  framed: number;
  categories: PlaybookCategory[];
  tempo: TacticalHistogram;
  sites: PlaybookSite[];
  zones: PlaybookZone[];
  tactics: PlaybookTactic[];
  combo: PlaybookCombo;
  rounds: PlaybookRound[];
};

/** A window of seconds after freeze end; `to_seconds` 0 has no end. */
export type PrepWindow = { from_seconds: number; to_seconds: number };

/** `examples` are the first rounds; `refs` holds every round, to open each in the replay. */
export type PrepRounds = ScoutRounds & { refs: ScoutEvidence[] };

export type PrepMatchup = PrepRounds & { side: TacticalSide; buy: TacticalBuyType; opponent_buy: TacticalBuyType };

export type PrepTempo = PrepRounds & {
  side: TacticalSide;
  buy: TacticalBuyType;
  tempo: TacticalHistogram;
  plant: TacticalHistogram;
};

/** Per round, the callout of most of the player's frames in the window; top 5. */
export type PrepPlayerWindow = PrepWindow & { rounds: number; spots: PrepSpot[] };

/** `pos` is the world centroid of the samples that named the spot; `floor` is "lower" on the lower radar of a two-level map. */
export type PrepSpot = ScoutSpot & { refs: ScoutEvidence[]; pos: [number, number, number]; floor?: 'lower' };

export type PrepPlayer = {
  steamid64: string;
  name: string;
  /** `library.Role*`: awp, entry, support, rifler or unknown. */
  role: string;
  rounds: number;
  windows: PrepPlayerWindow[];
};

export type PrepThresholds = {
  windows: PrepWindow[];
  spot_limit: number;
  example_limit: number;
  site_contact_players: number;
  tempo_bucket_seconds: number;
};

export type TeamPrep = {
  team_id: string;
  team_name: string;
  skipped: { match_id: string; reason: ScoutSkipReason }[];
  map: string;
  thresholds: PrepThresholds;
  matches: number;
  round_count: number;
  framed: number;
  matchups: PrepMatchup[];
  tempo: PrepTempo[];
  ct_players: PrepPlayer[];
};

export const PLAYBOOK_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'with', 'without'] as const;
export const PREP_QUERY = ['map', 'from', 'to', 'opponent_id', 'match_id'] as const;
export const PLAYBOOK_KEYS = [
  'team_id', 'team_name', 'skipped', 'map', 'side', 'thresholds', 'matches', 'round_count', 'win_rate', 'framed',
  'categories', 'tempo', 'sites', 'zones', 'tactics', 'combo', 'rounds',
] as const;
export const PREP_KEYS = [
  'team_id', 'team_name', 'skipped', 'map', 'thresholds', 'matches', 'round_count', 'framed', 'matchups', 'tempo', 'ct_players',
] as const;

export type TeamPlaybookQuery = {
  map: string;
  side: TacticalSide;
  from?: string;
  to?: string;
  opponent_ids?: readonly string[];
  match_ids?: readonly string[];
};

export type TeamPrepQuery = {
  map: string;
  from?: string;
  to?: string;
  opponent_ids?: readonly string[];
  match_ids?: readonly string[];
};
