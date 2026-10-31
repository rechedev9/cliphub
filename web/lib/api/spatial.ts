import type {
  TacticalBuyType,
  TacticalCTPattern,
  TacticalEventKind,
  TacticalGeometry,
  TacticalSide,
  TacticalSite,
  TacticalTPattern,
} from './tactical.ts';
import type { ScoutEvidence } from './scout.ts';

/**
 * Browser-side contract of the multi-round replay: the heatmaps and overlays of
 * `tacticalplan` (spatial.go) wrapped by `httpapi.spatialResponse`, and the
 * replay notes of `library.Annotation`. Field names are the Go JSON names.
 */

export const HEATMAP_KINDS = ['position', 'death', 'utility', 'timing'] as const;
export type HeatmapKind = (typeof HEATMAP_KINDS)[number];

/** The grenades a utility heatmap can count (`isScoutUtility`). */
export const HEATMAP_GRENADES = ['smoke', 'flash', 'molotov', 'he'] as const;
export type HeatmapGrenade = (typeof HEATMAP_GRENADES)[number];

export type HeatmapWindow = { from: number; to: number };

export type HeatmapCell = {
  side: TacticalSide;
  level: string;
  x: number;
  y: number;
  count: number;
  rounds: number;
  /** Median seconds after freeze end; for timing, the median first arrival. */
  seconds: number;
  examples: ScoutEvidence[];
};

export type HeatmapSideTotal = { side: TacticalSide; level: string; total: number; rounds: number; max_count: number };

export type Heatmap = {
  kind: HeatmapKind;
  window: HeatmapWindow;
  grenades?: TacticalEventKind[];
  cell_size: number;
  step_seconds: number;
  example_limit: number;
  matches: number;
  rounds: number;
  missing_positions: number;
  sides: HeatmapSideTotal[];
  cells: HeatmapCell[];
};

export type OverlayTrack = {
  slot: number;
  steamid64: string;
  name: string;
  side: TacticalSide;
  /** Index of the first point, in steps from freeze end. */
  start: number;
  points: [number, number, number][];
  died: boolean;
};

export type OverlayEvent = {
  seconds: number;
  tick: number;
  kind: TacticalEventKind;
  side?: TacticalSide;
  pos: [number, number, number];
  /** The followed players did it, or died, for a kill. */
  subject: boolean;
};

export type OverlayRound = {
  match_id: string;
  round: number;
  map: string;
  side?: TacticalSide;
  winner: TacticalSide | '';
  buy?: TacticalBuyType;
  t_pattern: TacticalTPattern;
  ct_pattern: TacticalCTPattern;
  site: TacticalSite;
  tick_freeze_end: number;
  tickrate: number;
  duration_seconds: number;
  tracks: OverlayTrack[];
  events: OverlayEvent[];
};

export type Overlay = {
  step_seconds: number;
  limit: number;
  total: number;
  missing_positions: number;
  rounds: OverlayRound[];
};

export type SpatialMatchRef = { match_id: string; map: string; played_at: string; team_key?: string; opponent?: string };

export type SpatialSkipped = { match_id: string; reason: string };

export type SpatialResponse = {
  team_id?: string;
  team_name?: string;
  map: string;
  maps: { map: string; matches: number }[];
  matches: SpatialMatchRef[];
  skipped: SpatialSkipped[];
  reference_match_id: string | null;
  geometry: TacticalGeometry | null;
  heatmap?: Heatmap;
  overlay?: Overlay;
};

/** Response keys the proxy lets through for a heatmap or an overlay. */
export const SPATIAL_RESPONSE_KEYS = [
  'team_id', 'team_name', 'map', 'maps', 'matches', 'skipped', 'reference_match_id', 'geometry', 'heatmap', 'overlay',
] as const;

/** Query names of the team views: the scout filter, the round filter and the view's own. */
export const TEAM_SPATIAL_QUERY = [
  'map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'buy', 'opponent_buy', 'site', 'outcome', 't_pattern',
  'ct_pattern', 'tag', 'phase',
] as const;
export const HEATMAP_QUERY = ['kind', 't_from', 't_to', 'grenade'] as const;
export const OVERLAY_QUERY = ['limit'] as const;

/* Replay notes ------------------------------------------------------------ */

/** Colour tokens a stroke may name (`library.StrokeColors`). */
export const STROKE_COLORS = ['accent', 'ct', 't', 'mark'] as const;
export type StrokeColor = (typeof STROKE_COLORS)[number];

/** One line of a drawing, its points in world x and y. */
export type Stroke = { color: StrokeColor; width: number; arrow: boolean; points: [number, number][] };

export type Annotation = {
  id: string;
  match_id: string;
  round: number;
  tick: number;
  text: string;
  strokes: Stroke[];
  created_at: string;
  updated_at: string;
};

export const ANNOTATION_KEYS = ['id', 'match_id', 'round', 'tick', 'text', 'strokes', 'created_at', 'updated_at'] as const;

/** Limits the orchestrator enforces (`library.CleanAnnotation`). */
export const ANNOTATION_TEXT_MAX = 2000;
export const ANNOTATION_STROKES_MAX = 64;
export const STROKE_POINTS_MAX = 2000;
