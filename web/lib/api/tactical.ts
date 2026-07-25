import { SERVICE_UNAVAILABLE_CODE } from './types.ts';

/**
 * Browser-side contract for the deterministic tactical analysis of a demo.
 *
 * Every type here mirrors a Go struct in `internal/tacticalplan` (plus
 * `internal/radarmap` for the radar transform), field for field, using the JSON
 * names those structs are tagged with. The Go package is the source of truth:
 * when a shape changes there, it changes here and nowhere else, so no component
 * ever re-declares a response shape.
 *
 * All access is same-origin through the `/api/demos/{jobId}/tactical/*` proxy
 * routes; the orchestrator URL and token never reach the browser.
 */

/** Document format understood by this client (`tacticalplan.SchemaVersion`). */
export const TACTICAL_SCHEMA_VERSION = '1.0';

/**
 * Round count below which a rate is reported but flagged as unreliable
 * (`tacticalplan.MinReliableSample`). A 1-of-1 site take is not a tendency.
 */
export const MIN_RELIABLE_SAMPLE = 4;

/* -------------------------------------------------------------------------- */
/* Closed vocabularies                                                        */
/* -------------------------------------------------------------------------- */

/** The two playing sides. Spectators never appear in a tactical document. */
export const TACTICAL_SIDES = { ct: 'CT', t: 'T' } as const;
export type TacticalSide = (typeof TACTICAL_SIDES)[keyof typeof TACTICAL_SIDES];

/** A team's economic state for one round (`tacticalplan.BuyType`). */
export const TACTICAL_BUY_TYPES = {
  pistol: 'pistol',
  eco: 'eco',
  semi: 'semi',
  force: 'force',
  full: 'full',
  unknown: 'unknown',
} as const;
export type TacticalBuyType = (typeof TACTICAL_BUY_TYPES)[keyof typeof TACTICAL_BUY_TYPES];

/** Buy types in economic order, matching `tacticalplan.BuyTypes`. */
export const TACTICAL_BUY_TYPE_ORDER: readonly TacticalBuyType[] = [
  TACTICAL_BUY_TYPES.pistol,
  TACTICAL_BUY_TYPES.eco,
  TACTICAL_BUY_TYPES.semi,
  TACTICAL_BUY_TYPES.force,
  TACTICAL_BUY_TYPES.full,
  TACTICAL_BUY_TYPES.unknown,
];

/** The attacking side's round shape (`tacticalplan.TPattern`). */
export const TACTICAL_T_PATTERNS = {
  execute: 'execute',
  default: 'default',
  split: 'split',
  fast: 'fast',
  ecoRush: 'eco_rush',
  save: 'save',
  unknown: 'unknown',
} as const;
export type TacticalTPattern = (typeof TACTICAL_T_PATTERNS)[keyof typeof TACTICAL_T_PATTERNS];

/** T patterns in the order the aggregate reports them. */
export const TACTICAL_T_PATTERN_ORDER: readonly TacticalTPattern[] = [
  TACTICAL_T_PATTERNS.execute,
  TACTICAL_T_PATTERNS.default,
  TACTICAL_T_PATTERNS.split,
  TACTICAL_T_PATTERNS.fast,
  TACTICAL_T_PATTERNS.ecoRush,
  TACTICAL_T_PATTERNS.save,
  TACTICAL_T_PATTERNS.unknown,
];

/** The defending side's round shape (`tacticalplan.CTPattern`). */
export const TACTICAL_CT_PATTERNS = {
  hold: 'hold',
  retake: 'retake',
  aggression: 'aggression',
  stack: 'stack',
  save: 'save',
  unknown: 'unknown',
} as const;
export type TacticalCTPattern = (typeof TACTICAL_CT_PATTERNS)[keyof typeof TACTICAL_CT_PATTERNS];

/** CT patterns in the order the aggregate reports them. */
export const TACTICAL_CT_PATTERN_ORDER: readonly TacticalCTPattern[] = [
  TACTICAL_CT_PATTERNS.hold,
  TACTICAL_CT_PATTERNS.retake,
  TACTICAL_CT_PATTERNS.aggression,
  TACTICAL_CT_PATTERNS.stack,
  TACTICAL_CT_PATTERNS.save,
  TACTICAL_CT_PATTERNS.unknown,
];

/** A bombsite, the neutral middle, or no site commitment at all. */
export const TACTICAL_SITES = { a: 'A', b: 'B', mid: 'mid', none: 'none' } as const;
export type TacticalSite = (typeof TACTICAL_SITES)[keyof typeof TACTICAL_SITES];

/** Sites in the order the aggregate reports them. */
export const TACTICAL_SITE_ORDER: readonly TacticalSite[] = [
  TACTICAL_SITES.a,
  TACTICAL_SITES.b,
  TACTICAL_SITES.mid,
  TACTICAL_SITES.none,
];

/**
 * Round events carried in the index (`tacticalplan.EventKind`). Per-tick
 * movement lives in the position blob, never here.
 */
export const TACTICAL_EVENT_KINDS = {
  kill: 'kill',
  plant: 'plant',
  defuse: 'defuse',
  explode: 'explode',
  smoke: 'smoke',
  flash: 'flash',
  he: 'he',
  molotov: 'molotov',
  decoy: 'decoy',
  bombDrop: 'bomb_drop',
} as const;
export type TacticalEventKind = (typeof TACTICAL_EVENT_KINDS)[keyof typeof TACTICAL_EVENT_KINDS];

/** Orthogonal round facts that do not fit the pattern vocabulary (the Tag* consts). */
export const TACTICAL_ROUND_TAGS = {
  postPlant: 'postplant',
  retakeWon: 'retake_won',
  fullSave: 'full_save',
  ace: 'ace',
  openingTraded: 'opening_traded',
  antiEco: 'anti_eco',
  overtime: 'overtime',
  pistol: 'pistol',
  timeExpired: 'time_expired',
} as const;
export type TacticalRoundTag = (typeof TACTICAL_ROUND_TAGS)[keyof typeof TACTICAL_ROUND_TAGS];

/** Round outcome, seen from the filter's perspective (`tacticalplan.Outcome`). */
export const TACTICAL_OUTCOMES = { win: 'win', loss: 'loss' } as const;
export type TacticalOutcome = (typeof TACTICAL_OUTCOMES)[keyof typeof TACTICAL_OUTCOMES];

/** Regulation or overtime rounds (`tacticalplan.Phase`). */
export const TACTICAL_PHASES = { regulation: 'regulation', overtime: 'overtime' } as const;
export type TacticalPhase = (typeof TACTICAL_PHASES)[keyof typeof TACTICAL_PHASES];

/** Vertical section a world position belongs to (`radarmap.Level*`). */
export const RADAR_LEVELS = { default: 'default', lower: 'lower' } as const;
export type RadarLevel = (typeof RADAR_LEVELS)[keyof typeof RADAR_LEVELS];

/**
 * How a calibration was obtained (`radarmap.Source*`): transcribed from the
 * map's shipped overview file, or derived from the positions observed in this
 * demo (stable inside the demo, not comparable with another one).
 */
export const RADAR_CALIBRATION_SOURCES = { overview: 'overview', derived: 'derived' } as const;
export type RadarCalibrationSource =
  (typeof RADAR_CALIBRATION_SOURCES)[keyof typeof RADAR_CALIBRATION_SOURCES];

/** The only map shape a demo can prove (`tacticalplan.GeometrySourceOccupancy`). */
export const GEOMETRY_SOURCE_OCCUPANCY = 'occupancy';

/**
 * Lifecycle of the tactical analysis of one job (`artifacts.TacticalState*`).
 * `none` is never stored: it is what the status endpoint reports for a job whose
 * analysis has not been requested yet.
 */
export const TACTICAL_STATES = {
  none: 'none',
  queued: 'queued',
  running: 'running',
  ready: 'ready',
  failed: 'failed',
} as const;
export type TacticalState = (typeof TACTICAL_STATES)[keyof typeof TACTICAL_STATES];

/** Position sampling rate the scan uses when the start request names none (`tactical.DefaultSampleHZ`). */
export const TACTICAL_DEFAULT_SAMPLE_HZ = 8;
/** Highest sampling rate the scan accepts (`tactical.MaxSampleHZ`); higher rates multiply the blob. */
export const TACTICAL_MAX_SAMPLE_HZ = 64;
/** Body field of the start request that selects the sampling rate. */
export const TACTICAL_SAMPLE_HZ_FIELD = 'sample_hz';

/* -------------------------------------------------------------------------- */
/* Position blob layout (zvpos1)                                              */
/* -------------------------------------------------------------------------- */

/** Sidecar blob layout name (`tacticalplan.PositionsFormat`). */
export const POSITIONS_FORMAT = 'zvpos1';
/** First six bytes of the blob. */
export const POSITIONS_MAGIC = 'ZVPOS1';
/** Blob version this client decodes. */
export const POSITIONS_VERSION = 1;
/** Fixed header size in bytes. */
export const POSITIONS_HEADER_SIZE = 32;
/** Per-frame header: int32 tick + uint16 present mask. */
export const POSITIONS_FRAME_HEAD_SIZE = 6;
/** Per-sample record: int16 x/y/z + uint16 yaw + uint8 health + uint8 flags. */
export const POSITIONS_SAMPLE_SIZE = 10;
/** Slots addressable by the 16-bit present mask. */
export const POSITIONS_MAX_SLOTS = 16;

/**
 * Per-sample boolean facts packed into one byte (`tacticalplan.SampleFlags`).
 * `sideT` marks the terrorist side so a dot can be coloured without consulting
 * the round's player list.
 */
export const TACTICAL_SAMPLE_FLAGS = {
  alive: 1 << 0,
  blinded: 1 << 1,
  ducking: 1 << 2,
  scoped: 1 << 3,
  hasBomb: 1 << 4,
  defusing: 1 << 5,
  airborne: 1 << 6,
  sideT: 1 << 7,
} as const;
export type TacticalSampleFlag = (typeof TACTICAL_SAMPLE_FLAGS)[keyof typeof TACTICAL_SAMPLE_FLAGS];

/** Reports whether every bit in `mask` is set, like `SampleFlags.Has`. */
export function hasSampleFlags(flags: number, mask: number): boolean {
  return (flags & mask) === mask;
}

/* -------------------------------------------------------------------------- */
/* Document                                                                   */
/* -------------------------------------------------------------------------- */

/** The top-level tactical analysis of one demo (`tacticalplan.Document`). */
export type TacticalDocument = {
  schema_version: string;
  generated_at: string;
  job_id?: string;
  demo: TacticalDemo;
  teams: TacticalTeam[];
  players: TacticalPlayer[];
  rounds: TacticalRound[];
  geometry: TacticalGeometry;
  positions: TacticalPositions;
  /** Every ambiguity the scan resolved by convention. */
  warnings?: string[];
};

/** The source demo's identity and timing (`tacticalplan.Demo`). */
export type TacticalDemo = {
  path: string;
  sha256: string;
  map: string;
  tickrate: number;
  duration_ticks: number;
  format: string;
  /** Detected regulation length: 24 for MR12, 30 for MR15. */
  max_rounds: number;
  overtime_rounds?: number;
  regulation_ended_round?: number;
};

/**
 * A team identity (`tacticalplan.Team`). Sides swap at halftime, so a team is
 * identified by clan name and fielded players, never by side.
 */
export type TacticalTeam = {
  key: string;
  name: string;
  start_side: TacticalSide;
  slots: number[];
  steamids: string[];
};

/**
 * The identity table (`tacticalplan.Player`). Every sample and every event
 * refers to `slot`, the only stable key across a name change.
 */
export type TacticalPlayer = {
  slot: number;
  steamid64: string;
  name: string;
  team_key: string;
  start_side: TacticalSide;
};

/** One played round (`tacticalplan.Round`). `winner` is empty on a round with no result. */
export type TacticalRound = {
  number: number;
  tick_start: number;
  tick_freeze_end: number;
  tick_end: number;
  tick_official: number;
  score_ct_before: number;
  score_t_before: number;
  winner: TacticalSide | '';
  end_reason: string;
  half: number;
  overtime?: number;
  /** Absent on a round with no plant. */
  bomb?: TacticalBomb;
  economy: TacticalEconomy;
  class: TacticalClass;
  players: TacticalPlayerRound[];
  events: TacticalEvent[];
};

/** The bomb's fate (`tacticalplan.Bomb`). */
export type TacticalBomb = {
  plant_tick: number;
  site: TacticalSite;
  planter_slot?: number;
  defuse_tick?: number;
  defuser_slot?: number;
  explode_tick?: number;
};

/** Both sides' buy state, sampled once per round (`tacticalplan.Economy`). */
export type TacticalEconomy = {
  ct_equip_value: number;
  t_equip_value: number;
  ct_money: number;
  t_money: number;
  ct_buy: TacticalBuyType;
  t_buy: TacticalBuyType;
  /** Freeze-time end plus a fixed delay: players keep buying after it. */
  sample_tick: number;
};

/**
 * The deterministic round taxonomy (`tacticalplan.Class`). `reasons` records why
 * the classifier landed where it did, so a rule change is a reviewable diff.
 */
export type TacticalClass = {
  t_side: TacticalTPattern;
  ct_side: TacticalCTPattern;
  site: TacticalSite;
  opening_slot?: number;
  opening_side?: TacticalSide;
  opening_tick?: number;
  opening_traded: boolean;
  /** The round's first kill: when map control turned into a fight. */
  first_contact_tick?: number;
  tags: string[];
  reasons: string[];
};

/** One player's round (`tacticalplan.PlayerRound`). */
export type TacticalPlayerRound = {
  slot: number;
  side: TacticalSide;
  kills: number;
  deaths: number;
  assists: number;
  damage: number;
  equip_value: number;
  money: number;
  death_tick?: number;
  survived: boolean;
  opening_kill?: boolean;
  opening_death?: boolean;
  /** This player died and a teammate killed the killer inside the trade window. */
  traded?: boolean;
  trade_kills?: number;
};

/**
 * One thing that happened at a tick (`tacticalplan.Event`). Positions are world
 * coordinates; the reader transforms them to radar pixels.
 *
 * `target_pos` is tagged `omitempty` in Go, but Go never omits a fixed-size
 * array, so it is always present (zeroed when there is no target).
 */
export type TacticalEvent = {
  tick: number;
  kind: TacticalEventKind;
  actor_slot?: number;
  target_slot?: number;
  side?: TacticalSide;
  weapon?: string;
  pos: [number, number, number];
  target_pos: [number, number, number];
  place?: string;
  site?: TacticalSite;
  headshot?: boolean;
  wallbang?: boolean;
  through_smoke?: boolean;
  attacker_blind?: boolean;
  no_scope?: boolean;
  traded?: boolean;
  opening?: boolean;
};

/* -------------------------------------------------------------------------- */
/* Geometry and radar calibration                                             */
/* -------------------------------------------------------------------------- */

/**
 * The drawable map (`tacticalplan.MapGeometry`), derived from play rather than
 * from game assets, so a viewer can draw a faithful licence-free radar with no
 * external image.
 */
export type TacticalGeometry = {
  map: string;
  source: string;
  calibration: RadarCalibration;
  bounds: RadarBounds;
  cell_size: number;
  levels: TacticalGeometryLevel[];
  callouts: TacticalCallout[];
  sample_count: number;
};

/** A packed occupancy cell: `[cellX, cellY, weight]`. */
export type TacticalGeometryCell = [number, number, number];

/** One vertical section's occupancy grid (`tacticalplan.GeometryLevel`). */
export type TacticalGeometryLevel = {
  name: RadarLevel;
  cells: TacticalGeometryCell[];
};

/** One named position and the centre of mass of play observed in it. */
export type TacticalCallout = {
  name: string;
  level: RadarLevel;
  center: [number, number];
  samples: number;
};

/**
 * One map's overview transform (`radarmap.Calibration`). `pos_x`/`pos_y` are the
 * world coordinate of the radar image's upper-left corner and `scale` is world
 * units per native radar pixel, exactly as CS2 defines them.
 */
export type RadarCalibration = {
  map: string;
  source: RadarCalibrationSource;
  pos_x: number;
  pos_y: number;
  scale: number;
  /** Native radar resolution in pixels; 1024 or 2048, always read with `scale`. */
  size: number;
  /** AltitudeMax of the lower section on split-level maps; absent on single-level maps. */
  lower_altitude_max?: number;
};

/** An axis-aligned world-space rectangle (`radarmap.Bounds`). */
export type RadarBounds = {
  min_x: number;
  min_y: number;
  max_x: number;
  max_y: number;
};

/* -------------------------------------------------------------------------- */
/* Positions sidecar                                                          */
/* -------------------------------------------------------------------------- */

/**
 * Describes the sidecar blob (`tacticalplan.Positions`): how it was sampled,
 * how to decode it, and how to seek into it. It travels inside the JSON
 * document; the bytes it describes do not.
 */
export type TacticalPositions = {
  format: string;
  hz: number;
  sample_ticks: number;
  quantum: number;
  origin: [number, number, number];
  slot_count: number;
  frame_count: number;
  byte_length: number;
  /** Binds the index to its blob; a mismatch means the analysis is stale. */
  sha256: string;
  round_offsets: TacticalRoundOffset[];
};

/** Locates one round's frames inside the blob (`tacticalplan.RoundOffset`). */
export type TacticalRoundOffset = {
  round: number;
  byte_offset: number;
  byte_length: number;
  frame_count: number;
  first_tick: number;
  last_tick: number;
};

/**
 * One player's state at one sampled tick (`tacticalplan.Sample`). The Go struct
 * carries no JSON tags, so the wire names are its Go field names; the binary
 * decoder produces exactly this shape too, so both paths agree.
 */
export type TacticalSample = {
  slot: number;
  x: number;
  y: number;
  z: number;
  /** Degrees counter-clockwise from +X, as CS2 reports it. */
  yaw: number;
  health: number;
  flags: number;
};

/** Every sampled player at one tick (`tacticalplan.Frame`). */
export type TacticalFrame = {
  tick: number;
  samples: TacticalSample[];
};

/* -------------------------------------------------------------------------- */
/* Filter, aggregate, status                                                  */
/* -------------------------------------------------------------------------- */

/**
 * Selects rounds (`tacticalplan.Filter`). Fields compose with AND; values inside
 * a field compose with OR. Set `team_key` to follow one team across the side
 * swap, or `side` to look at one side of the server regardless of who was on it.
 */
export type TacticalFilter = {
  team_key?: string;
  side?: TacticalSide;
  buys?: TacticalBuyType[];
  opponent_buys?: TacticalBuyType[];
  sites?: TacticalSite[];
  outcome?: TacticalOutcome;
  t_patterns?: TacticalTPattern[];
  ct_patterns?: TacticalCTPattern[];
  tags?: string[];
  slots?: number[];
  round_from?: number;
  round_to?: number;
  phase?: TacticalPhase;
};

/**
 * Query-parameter names the aggregate endpoint filters by, exactly as
 * `tacticalplan.FilterFromValues` reads them. Repeated values are ORed.
 */
export const TACTICAL_FILTER_PARAMS = {
  team: 'team',
  side: 'side',
  buy: 'buy',
  opponentBuy: 'opponent_buy',
  site: 'site',
  outcome: 'outcome',
  tPattern: 't_pattern',
  ctPattern: 'ct_pattern',
  tag: 'tag',
  slot: 'slot',
  roundFrom: 'round_from',
  roundTo: 'round_to',
  phase: 'phase',
} as const;
export type TacticalFilterParam =
  (typeof TACTICAL_FILTER_PARAMS)[keyof typeof TACTICAL_FILTER_PARAMS];

/** Every filter parameter name, for whitelisting an incoming query string. */
export const TACTICAL_FILTER_PARAM_NAMES: readonly TacticalFilterParam[] = [
  TACTICAL_FILTER_PARAMS.team,
  TACTICAL_FILTER_PARAMS.side,
  TACTICAL_FILTER_PARAMS.buy,
  TACTICAL_FILTER_PARAMS.opponentBuy,
  TACTICAL_FILTER_PARAMS.site,
  TACTICAL_FILTER_PARAMS.outcome,
  TACTICAL_FILTER_PARAMS.tPattern,
  TACTICAL_FILTER_PARAMS.ctPattern,
  TACTICAL_FILTER_PARAMS.tag,
  TACTICAL_FILTER_PARAMS.slot,
  TACTICAL_FILTER_PARAMS.roundFrom,
  TACTICAL_FILTER_PARAMS.roundTo,
  TACTICAL_FILTER_PARAMS.phase,
];

/** A count over a denominator (`tacticalplan.Rate`). Never render `pct` without `total`. */
export type TacticalRate = {
  count: number;
  total: number;
  pct: number;
  /** False when `total` is below MIN_RELIABLE_SAMPLE. */
  reliable: boolean;
};

/** One economic state and how the perspective fared in it (`tacticalplan.BuyBucket`). */
export type TacticalBuyBucket = {
  buy: TacticalBuyType;
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
  conversion: TacticalRate;
};

/** The perspective's buy crossed with the opponent's: the correct "anti-eco". */
export type TacticalMatchupBucket = {
  buy: TacticalBuyType;
  opponent_buy: TacticalBuyType;
  rounds: number;
  win_rate: TacticalRate;
};

/** Where the round was decided and how it went (`tacticalplan.SiteBucket`). */
export type TacticalSiteBucket = {
  site: TacticalSite;
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
};

/** On this economy, where do they go, and does it work (`tacticalplan.BuySiteBucket`). */
export type TacticalBuySiteBucket = {
  buy: TacticalBuyType;
  site: TacticalSite;
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
};

/** One round shape's frequency and success (`tacticalplan.PatternBucket`). */
export type TacticalPatternBucket = {
  pattern: string;
  rounds: number;
  share: TacticalRate;
  win_rate: TacticalRate;
};

/** The opening duel (`tacticalplan.OpeningSummary`), the most predictive round event. */
export type TacticalOpeningSummary = {
  rounds: number;
  won: TacticalRate;
  traded_on_loss: TacticalRate;
  round_win_after_opening_win: TacticalRate;
  round_win_after_opening_loss: TacticalRate;
};

/** When things happened, in seconds from freeze-time end (`tacticalplan.TimingSummary`). */
export type TacticalTimingSummary = {
  first_contact: TacticalHistogram;
  plant: TacticalHistogram;
  round_duration: TacticalHistogram;
};

/** A bucketed distribution with the median kept alongside (`tacticalplan.Histogram`). */
export type TacticalHistogram = {
  bucket_seconds: number;
  samples: number;
  median: number;
  buckets: TacticalHistogramBucket[];
};

/** Counts samples in `[from_seconds, from_seconds + bucket_seconds)`. */
export type TacticalHistogramBucket = {
  from_seconds: number;
  count: number;
};

/** One player's contribution across the filtered rounds (`tacticalplan.PlayerSummary`). */
export type TacticalPlayerSummary = {
  slot: number;
  name: string;
  rounds: number;
  kills: number;
  deaths: number;
  assists: number;
  damage: number;
  adr: number;
  opening_kills: number;
  opening_deaths: number;
  trade_kills: number;
  survival_rate: TacticalRate;
};

/**
 * The aggregate answer to "what does this team do" over the rounds a filter
 * selected (`tacticalplan.Tendencies`). Every rate ships with its counts.
 */
export type TacticalTendencies = {
  filter: TacticalFilter;
  round_count: number;
  /** The side the rates are computed from when the filter fixes one. */
  perspective?: TacticalSide;
  wins: number;
  buys: TacticalBuyBucket[];
  matchups: TacticalMatchupBucket[];
  sites: TacticalSiteBucket[];
  buy_sites: TacticalBuySiteBucket[];
  t_patterns: TacticalPatternBucket[];
  ct_patterns: TacticalPatternBucket[];
  openings: TacticalOpeningSummary;
  timings: TacticalTimingSummary;
  players: TacticalPlayerSummary[];
};

/** Lifecycle of a job's tactical analysis, from the status and start endpoints. */
export type TacticalStatus = {
  state: TacticalState;
  generated_at: string;
  schema_version: string;
  error?: string;
};

/** One round's index entry plus its decoded position frames. */
export type TacticalRoundFrames = {
  round: TacticalRound;
  frames: TacticalFrame[];
};

/* -------------------------------------------------------------------------- */
/* Proxy-route response whitelists                                            */
/* -------------------------------------------------------------------------- */

/** Top-level keys the tactical document proxy forwards. */
export const TACTICAL_DOCUMENT_KEYS: readonly string[] = [
  'schema_version',
  'generated_at',
  'job_id',
  'demo',
  'teams',
  'players',
  'rounds',
  'geometry',
  'positions',
  'warnings',
];

/** Top-level keys the status/start proxy forwards. */
export const TACTICAL_STATUS_KEYS: readonly string[] = [
  'state',
  'generated_at',
  'schema_version',
  'error',
];

/** Top-level keys the per-round proxy forwards. */
export const TACTICAL_ROUND_KEYS: readonly string[] = ['round', 'frames'];

/** Top-level keys the aggregate proxy forwards. */
export const TACTICAL_TENDENCIES_KEYS: readonly string[] = [
  'filter',
  'round_count',
  'perspective',
  'wins',
  'buys',
  'matchups',
  'sites',
  'buy_sites',
  't_patterns',
  'ct_patterns',
  'openings',
  'timings',
  'players',
];

/* -------------------------------------------------------------------------- */
/* Same-origin addressing                                                     */
/* -------------------------------------------------------------------------- */

/** Base path of the demo proxy routes; the orchestrator is never addressed directly. */
export const TACTICAL_API_BASE = '/api/demos';

function jobBase(jobId: string): string {
  return `${TACTICAL_API_BASE}/${encodeURIComponent(jobId)}/tactical`;
}

/** GET (read) / POST (start) the tactical document of a job. */
export function tacticalDocumentUrl(jobId: string): string {
  return jobBase(jobId);
}

/** GET the analysis lifecycle state of a job. */
export function tacticalStatusUrl(jobId: string): string {
  return `${jobBase(jobId)}/status`;
}

/** GET one round's index entry and position frames. */
export function tacticalRoundUrl(jobId: string, round: number): string {
  return `${jobBase(jobId)}/rounds/${encodeURIComponent(String(round))}`;
}

/** GET the raw zvpos1 blob; Range-capable. */
export function tacticalPositionsUrl(jobId: string): string {
  return `${jobBase(jobId)}/positions`;
}

/** GET the tendencies computed over the rounds a filter selects. */
export function tacticalAggregateUrl(jobId: string, filter: TacticalFilter = {}): string {
  const query = tacticalFilterParams(filter).toString();
  return query ? `${jobBase(jobId)}/aggregate?${query}` : `${jobBase(jobId)}/aggregate`;
}

/** Serializes a filter into the query parameters `FilterFromValues` parses. */
export function tacticalFilterParams(filter: TacticalFilter): URLSearchParams {
  const params = new URLSearchParams();
  if (filter.team_key) params.append(TACTICAL_FILTER_PARAMS.team, filter.team_key);
  if (filter.side) params.append(TACTICAL_FILTER_PARAMS.side, filter.side);
  for (const buy of filter.buys ?? []) params.append(TACTICAL_FILTER_PARAMS.buy, buy);
  for (const buy of filter.opponent_buys ?? []) params.append(TACTICAL_FILTER_PARAMS.opponentBuy, buy);
  for (const site of filter.sites ?? []) params.append(TACTICAL_FILTER_PARAMS.site, site);
  if (filter.outcome) params.append(TACTICAL_FILTER_PARAMS.outcome, filter.outcome);
  for (const pattern of filter.t_patterns ?? []) params.append(TACTICAL_FILTER_PARAMS.tPattern, pattern);
  for (const pattern of filter.ct_patterns ?? []) params.append(TACTICAL_FILTER_PARAMS.ctPattern, pattern);
  for (const tag of filter.tags ?? []) params.append(TACTICAL_FILTER_PARAMS.tag, tag);
  for (const slot of filter.slots ?? []) params.append(TACTICAL_FILTER_PARAMS.slot, String(slot));
  if (filter.round_from !== undefined) params.append(TACTICAL_FILTER_PARAMS.roundFrom, String(filter.round_from));
  if (filter.round_to !== undefined) params.append(TACTICAL_FILTER_PARAMS.roundTo, String(filter.round_to));
  if (filter.phase) params.append(TACTICAL_FILTER_PARAMS.phase, filter.phase);
  return params;
}

/** A byte range of the positions blob, inclusive of both ends, as HTTP Range is. */
export type TacticalByteRange = { start: number; endInclusive: number };

/** The range covering just the fixed blob header. */
export function tacticalPositionsHeaderRange(): TacticalByteRange {
  return { start: 0, endInclusive: POSITIONS_HEADER_SIZE - 1 };
}

/** The range covering exactly one round's frames, from its document offset. */
export function tacticalRoundByteRange(offset: TacticalRoundOffset): TacticalByteRange {
  return { start: offset.byte_offset, endInclusive: offset.byte_offset + offset.byte_length - 1 };
}

/* -------------------------------------------------------------------------- */
/* Typed fetches                                                              */
/* -------------------------------------------------------------------------- */

/** Throws an Error carrying any upstream `code` for a non-2xx proxy response. */
async function throwResponseError(res: Response): Promise<never> {
  const body = (await res.json().catch(() => null)) as { error?: unknown; code?: unknown } | null;
  const message = body && typeof body.error === 'string' ? body.error : `request failed (${res.status})`;
  const err = new Error(message) as Error & { code?: string };
  if (body && typeof body.code === 'string') err.code = body.code;
  throw err;
}

async function readJson<T>(res: Response): Promise<T> {
  if (!res.ok) await throwResponseError(res);
  return (await res.json()) as T;
}

/** True when an error thrown by this module means the local orchestrator is down. */
export function isServiceUnavailableError(error: unknown): boolean {
  return error instanceof Error && 'code' in error && error.code === SERVICE_UNAVAILABLE_CODE;
}

/** Reports whether this client understands a document's schema version. */
export function isSupportedTacticalSchema(version: string): boolean {
  return version === TACTICAL_SCHEMA_VERSION;
}

/**
 * POST — start the tactical analysis of a parsed demo. The scan is deterministic
 * and idempotent, so a second start while one is queued reports the existing
 * state. `sampleHz` overrides the default sampling rate, up to
 * TACTICAL_MAX_SAMPLE_HZ.
 */
export async function startTacticalAnalysis(jobId: string, sampleHz?: number): Promise<TacticalStatus> {
  const init: RequestInit =
    sampleHz === undefined
      ? { method: 'POST' }
      : {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ [TACTICAL_SAMPLE_HZ_FIELD]: sampleHz }),
        };
  return readJson<TacticalStatus>(await fetch(tacticalDocumentUrl(jobId), init));
}

/** GET — the analysis lifecycle state, cheap enough to poll. */
export async function fetchTacticalStatus(jobId: string): Promise<TacticalStatus> {
  return readJson<TacticalStatus>(await fetch(tacticalStatusUrl(jobId), { cache: 'no-store' }));
}

/**
 * GET — the whole tactical document. Rejects a schema version this client does
 * not understand rather than rendering a partial analysis.
 */
export async function fetchTacticalDocument(jobId: string): Promise<TacticalDocument> {
  const doc = await readJson<TacticalDocument>(
    await fetch(tacticalDocumentUrl(jobId), { cache: 'no-store' }),
  );
  if (!isSupportedTacticalSchema(doc.schema_version)) {
    throw new Error(
      `unsupported tactical schema ${doc.schema_version}, expected ${TACTICAL_SCHEMA_VERSION}`,
    );
  }
  return doc;
}

/** GET — one round's index entry plus its already-decoded frames. */
export async function fetchTacticalRound(jobId: string, round: number): Promise<TacticalRoundFrames> {
  return readJson<TacticalRoundFrames>(
    await fetch(tacticalRoundUrl(jobId, round), { cache: 'no-store' }),
  );
}

/** GET — tendencies over the rounds the filter selects. */
export async function fetchTacticalTendencies(
  jobId: string,
  filter: TacticalFilter = {},
): Promise<TacticalTendencies> {
  return readJson<TacticalTendencies>(
    await fetch(tacticalAggregateUrl(jobId, filter), { cache: 'no-store' }),
  );
}

/**
 * GET — the raw zvpos1 bytes. Pass a range to fetch only the header or a single
 * round, which is what `tacticalRoundByteRange` is for; the proxy streams the
 * body instead of buffering the whole blob.
 */
export async function fetchTacticalPositions(
  jobId: string,
  range?: TacticalByteRange,
): Promise<ArrayBuffer> {
  const headers: Record<string, string> = {};
  if (range) headers.Range = `bytes=${range.start}-${range.endInclusive}`;
  const res = await fetch(tacticalPositionsUrl(jobId), { cache: 'no-store', headers });
  if (!res.ok) await throwResponseError(res);
  return res.arrayBuffer();
}
