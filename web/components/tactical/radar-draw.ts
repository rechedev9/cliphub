import { RADAR_LEVELS, TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '@/lib/api/tactical';
import type {
  RadarLevel,
  TacticalEventKind,
  TacticalGeometry,
  TacticalGeometryLevel,
  TacticalSample,
  TacticalSide,
} from '@/lib/api/tactical';
import { cellRadarRect, maxCellWeight, worldToRendered, yawToRadarAngle } from '@/lib/tactical-transform';
import type { RadarPoint, RadarRect } from '@/lib/tactical-transform';
import type { TimelineEvent } from '@/lib/tactical-timeline';
import type { TrailPoint } from '@/lib/tactical-replay';
import { opponentSide } from '@/lib/tactical-labels';

/**
 * Every mark the 2D replay puts on the canvas.
 *
 * This module owns pixels and nothing else: it takes a scene of already-decoded
 * facts and paints it. It never reads the DOM, never fetches, and never decides
 * what is true — which keeps the drawing reviewable next to the transform it
 * mirrors (`internal/radarmap`, via `lib/tactical-transform`).
 *
 * Two passes exist on purpose. The map is play-derived geometry that only
 * changes with the size, the level or the theme, so it is baked into an
 * offscreen bitmap; the round is what changes 60 times a second. Inside the
 * per-frame pass the identity chips are painted last, after every blip: a name
 * composited under the next player's cone and dot is a name nobody can read.
 */

/** Colours and type, resolved once from the design tokens on the live element. */
export type RadarStyle = {
  background: string;
  panel: string;
  /** The ramp's raised step: the only foreground plate the map carries. */
  plate: string;
  map: string;
  callout: string;
  ct: string;
  t: string;
  bomb: string;
  defuse: string;
  utility: string;
  text: string;
  fontFamily: string;
};

/** One frame's worth of round state, in world coordinates. */
export type RadarScene = {
  /** Side of the virtual radar square, in CSS pixels; the plate shows a window onto it. */
  size: number;
  /**
   * The window of that square the plate actually shows, in fractions of it
   * (`radarViewRect`). Everything still draws in the square's coordinates; this
   * is only what the marks that have to stay ON SCREEN are clamped against.
   */
  view: RadarRect;
  geometry: TacticalGeometry;
  /** Level drawn at full strength; the others are dimmed in the background pass. */
  activeLevel: RadarLevel;
  samples: readonly TacticalSample[];
  trails: ReadonlyMap<number, readonly TrailPoint[]>;
  /** Events that have already happened at the playhead, oldest first. */
  events: readonly TimelineEvent[];
  /** Playhead position in bar seconds, used to fade older marks. */
  nowSeconds: number;
  /** Short display name per slot, drawn on a chip in a pass above every blip. */
  labels: ReadonlyMap<number, string>;
  style: RadarStyle;
};

/** Occupancy weight is long-tailed, so the ramp is on its square root. */
const CELL_MIN_ALPHA = 0.07;
const CELL_ALPHA_RANGE = 0.38;
/** Levels the player is not on stay legible but recede. */
const INACTIVE_LEVEL_ALPHA = 0.22;
/** Callouts below this share of the busiest one would only add noise. */
const CALLOUT_SAMPLE_SHARE = 0.12;
/** Below this size a callout label costs more legibility than it adds. */
const CALLOUT_MIN_SIZE = 380;
/** An event mark fades to its resting alpha over this many seconds. */
const EVENT_FRESH_SECONDS = 4;

/*
 * Identity chips. Position, side and facing are scanned every frame; a name is
 * read occasionally and never changes inside a round, so identity buys its
 * legibility from an opaque ramp step instead of from size, and it is painted
 * last. Occlusion reads; compositing does not.
 */
const TAG_MIN_SIZE = 380;
/** 12px is the system's hard type floor (--text-meta), not its target. */
const TAG_FONT_MIN = 12;
/** Above the callouts' 0.0145 at every size, so map furniture never out-sizes a name. */
const TAG_FONT_RATIO = 0.016;
const TAG_PAD_X = 4;
const TAG_RAIL = 2;
/**
 * Chip gap in blip radii: clears the health ring (1.6r — radius + lineWidth
 * plus half the stroke), the bomb dot (2.12r) and the defuse ring (2.46r).
 */
const TAG_GAP = 2.6;
/** A collided chip steps this far past the last one. */
const TAG_STACK_GAP = 3;
/** Outer edge of the health ring, in blip radii — where a leader starts. */
const TAG_RING_EDGE = 1.6;
/** A chip that had to move keeps a hairline back to its dot. */
const TAG_LEADER_ALPHA = 0.5;

function sideColor(style: RadarStyle, side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? style.ct : style.t;
}

/** Which side a sample is on. Read by the trails, the blip and the identity chip. */
function sampleSide(sample: TacticalSample): TacticalSide {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT)
    ? TACTICAL_SIDES.t
    : TACTICAL_SIDES.ct;
}

/** The dot's radius. Shared, so the chip pass cannot drift off the blip. */
function blipRadius(size: number): number {
  return Math.max(3.5, size * 0.0105);
}

function drawLevelCells(
  ctx: CanvasRenderingContext2D,
  geometry: TacticalGeometry,
  level: TacticalGeometryLevel,
  size: number,
  style: RadarStyle,
  strength: number,
): void {
  const max = maxCellWeight(level);
  if (max <= 0) return;
  ctx.fillStyle = style.map;
  for (const cell of level.cells) {
    const rect = cellRadarRect(geometry, cell, size);
    ctx.globalAlpha = (CELL_MIN_ALPHA + CELL_ALPHA_RANGE * Math.sqrt(cell[2] / max)) * strength;
    // Cells tile the world grid exactly; the half-pixel bleed keeps a seam from
    // showing between two neighbours at fractional scales.
    ctx.fillRect(rect.x, rect.y, rect.width + 0.5, rect.height + 0.5);
  }
  ctx.globalAlpha = 1;
}

function drawCallouts(
  ctx: CanvasRenderingContext2D,
  geometry: TacticalGeometry,
  size: number,
  style: RadarStyle,
  activeLevel: RadarLevel,
): void {
  if (size < CALLOUT_MIN_SIZE || geometry.callouts.length === 0) return;
  const busiest = geometry.callouts.reduce((max, callout) => Math.max(max, callout.samples), 0);
  if (busiest <= 0) return;

  // Static map furniture sits BEHIND the blips, so it is set smaller than the
  // identity chips (0.016, floored at 12px) rather than larger.
  ctx.font = `${Math.round(size * 0.0145)}px ${style.fontFamily}`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillStyle = style.callout;
  for (const callout of geometry.callouts) {
    if (callout.samples / busiest < CALLOUT_SAMPLE_SHARE) continue;
    if (callout.level !== activeLevel) continue;
    const point = worldToRendered(geometry.calibration, callout.center[0], callout.center[1], size);
    ctx.globalAlpha = 0.32;
    ctx.fillText(callout.name.toUpperCase(), point.x, point.y);
  }
  ctx.globalAlpha = 1;
}

/**
 * Bakes the visible window of the play-derived map: every level's occupancy
 * grid, the active one at full strength, plus the callout labels. The bitmap is
 * device-resolution and is exactly the plate's box rather than the whole square,
 * so it does not grow as the crop zooms in. `view` is a window in fractions of
 * the square (see `radarViewRect`), never a change of transform.
 */
export function renderRadarBackground(
  geometry: TacticalGeometry,
  activeLevel: RadarLevel,
  size: number,
  dpr: number,
  style: RadarStyle,
  view: RadarRect,
): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(view.width * size * dpr));
  canvas.height = Math.max(1, Math.round(view.height * size * dpr));
  const ctx = canvas.getContext('2d');
  if (ctx === null) return canvas;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  ctx.fillStyle = style.panel;
  ctx.fillRect(0, 0, view.width * size, view.height * size);
  // Everything below still draws in the `size` square's coordinates; this is the
  // only line that makes the bitmap a window onto it.
  ctx.translate(-view.x * size, -view.y * size);

  for (const level of geometry.levels) {
    if (level.name === activeLevel) continue;
    drawLevelCells(ctx, geometry, level, size, style, INACTIVE_LEVEL_ALPHA);
  }
  const active = geometry.levels.find((level) => level.name === activeLevel);
  if (active !== undefined) drawLevelCells(ctx, geometry, active, size, style, 1);
  else {
    // A demo with no cells on this level still deserves its default grid rather
    // than an empty square.
    const fallback = geometry.levels.find((level) => level.name === RADAR_LEVELS.default);
    if (fallback !== undefined) drawLevelCells(ctx, geometry, fallback, size, style, 1);
  }

  drawCallouts(ctx, geometry, size, style, activeLevel);
  return canvas;
}

function drawTrails(ctx: CanvasRenderingContext2D, scene: RadarScene): void {
  const { geometry, size, style } = scene;
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.lineWidth = Math.max(1.25, size * 0.0035);

  for (const sample of scene.samples) {
    const points = scene.trails.get(sample.slot);
    if (points === undefined || points.length < 2) continue;
    const side = sampleSide(sample);
    ctx.strokeStyle = sideColor(style, side);
    for (let i = 1; i < points.length; i += 1) {
      const from = worldToRendered(geometry.calibration, points[i - 1].x, points[i - 1].y, size);
      const to = worldToRendered(geometry.calibration, points[i].x, points[i].y, size);
      // The tail fades out, so the head of the trail reads as "now".
      ctx.globalAlpha = 0.08 + 0.34 * (i / (points.length - 1));
      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      ctx.lineTo(to.x, to.y);
      ctx.stroke();
    }
  }
  ctx.globalAlpha = 1;
}

function utilityColor(kind: TacticalEventKind, style: RadarStyle): string {
  switch (kind) {
    case TACTICAL_EVENT_KINDS.flash:
      return style.text;
    case TACTICAL_EVENT_KINDS.he:
    case TACTICAL_EVENT_KINDS.molotov:
      return style.bomb;
    default:
      return style.utility;
  }
}

function strokeCross(ctx: CanvasRenderingContext2D, x: number, y: number, arm: number): void {
  ctx.beginPath();
  ctx.moveTo(x - arm, y - arm);
  ctx.lineTo(x + arm, y + arm);
  ctx.moveTo(x + arm, y - arm);
  ctx.lineTo(x - arm, y + arm);
  ctx.stroke();
}

function strokeDiamond(ctx: CanvasRenderingContext2D, x: number, y: number, radius: number): void {
  ctx.beginPath();
  ctx.moveTo(x, y - radius);
  ctx.lineTo(x + radius, y);
  ctx.lineTo(x, y + radius);
  ctx.lineTo(x - radius, y);
  ctx.closePath();
}

function drawEvents(ctx: CanvasRenderingContext2D, scene: RadarScene): void {
  const { geometry, size, style } = scene;
  const mark = Math.max(3, size * 0.009);

  for (const entry of scene.events) {
    const { event } = entry;
    const age = scene.nowSeconds - entry.seconds;
    const freshness = 1 - Math.min(1, Math.max(0, age) / EVENT_FRESH_SECONDS);
    const alpha = 0.32 + 0.55 * freshness;
    const at = worldToRendered(geometry.calibration, event.pos[0], event.pos[1], size);
    ctx.lineWidth = Math.max(1.25, size * 0.003);

    if (event.kind === TACTICAL_EVENT_KINDS.kill) {
      const victimSide = event.side === undefined ? undefined : opponentSide(event.side);
      const victim = worldToRendered(geometry.calibration, event.target_pos[0], event.target_pos[1], size);
      if (event.side !== undefined) {
        ctx.strokeStyle = sideColor(style, event.side);
        ctx.globalAlpha = alpha * 0.35;
        ctx.beginPath();
        ctx.moveTo(at.x, at.y);
        ctx.lineTo(victim.x, victim.y);
        ctx.stroke();
      }
      ctx.strokeStyle = victimSide === undefined ? style.utility : sideColor(style, victimSide);
      ctx.globalAlpha = alpha;
      strokeCross(ctx, victim.x, victim.y, mark);
      continue;
    }

    if (event.kind === TACTICAL_EVENT_KINDS.plant || event.kind === TACTICAL_EVENT_KINDS.explode) {
      ctx.globalAlpha = alpha;
      ctx.strokeStyle = style.bomb;
      ctx.fillStyle = style.bomb;
      strokeDiamond(ctx, at.x, at.y, mark * 1.3);
      ctx.fill();
      ctx.globalAlpha = alpha * 0.5;
      ctx.beginPath();
      ctx.arc(at.x, at.y, mark * 2.4, 0, Math.PI * 2);
      ctx.stroke();
      continue;
    }

    if (event.kind === TACTICAL_EVENT_KINDS.defuse) {
      ctx.globalAlpha = alpha;
      ctx.strokeStyle = style.defuse;
      ctx.beginPath();
      ctx.arc(at.x, at.y, mark * 1.6, 0, Math.PI * 2);
      ctx.stroke();
      continue;
    }

    ctx.globalAlpha = alpha * 0.75;
    ctx.strokeStyle = utilityColor(event.kind, style);
    ctx.beginPath();
    ctx.arc(at.x, at.y, mark * 1.15, 0, Math.PI * 2);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
}

function drawPlayerBlip(ctx: CanvasRenderingContext2D, scene: RadarScene, sample: TacticalSample): void {
  const { geometry, size, style } = scene;
  const at = worldToRendered(geometry.calibration, sample.x, sample.y, size);
  const side = sampleSide(sample);
  const color = sideColor(style, side);
  const radius = blipRadius(size);

  if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) {
    ctx.globalAlpha = 0.4;
    ctx.strokeStyle = color;
    ctx.lineWidth = Math.max(1.25, size * 0.0028);
    strokeCross(ctx, at.x, at.y, radius * 0.9);
    ctx.globalAlpha = 1;
    return;
  }

  // View cone: where the player is actually looking, which is most of what a 2D
  // replay can say about intent.
  const heading = (yawToRadarAngle(sample.yaw) * Math.PI) / 180;
  const half = (26 * Math.PI) / 180;
  const reach = radius * 4.4;
  // Five cones overlapping at spawn used to composite into one solid mass.
  ctx.globalAlpha = 0.11;
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(at.x, at.y);
  ctx.arc(at.x, at.y, reach, heading - half, heading + half);
  ctx.closePath();
  ctx.fill();

  // Health ring, drawn clockwise from twelve o'clock.
  const health = Math.max(0, Math.min(100, sample.health));
  ctx.globalAlpha = 0.9;
  ctx.lineWidth = Math.max(1.5, radius * 0.4);
  ctx.strokeStyle = style.panel;
  ctx.beginPath();
  ctx.arc(at.x, at.y, radius + ctx.lineWidth, 0, Math.PI * 2);
  ctx.stroke();
  if (health > 0) {
    ctx.strokeStyle = color;
    ctx.beginPath();
    ctx.arc(
      at.x,
      at.y,
      radius + ctx.lineWidth,
      -Math.PI / 2,
      -Math.PI / 2 + (health / 100) * Math.PI * 2,
    );
    ctx.stroke();
  }

  ctx.globalAlpha = 1;
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.arc(at.x, at.y, radius, 0, Math.PI * 2);
  ctx.fill();

  if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb)) {
    ctx.fillStyle = style.bomb;
    ctx.beginPath();
    ctx.arc(at.x + radius * 1.5, at.y + radius * 1.5, radius * 0.62, 0, Math.PI * 2);
    ctx.fill();
  }
  if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.defusing)) {
    ctx.strokeStyle = style.defuse;
    ctx.lineWidth = Math.max(1.25, radius * 0.32);
    ctx.beginPath();
    ctx.arc(at.x, at.y, radius * 2.3, 0, Math.PI * 2);
    ctx.stroke();
  }
  if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.blinded)) {
    ctx.strokeStyle = style.text;
    ctx.globalAlpha = 0.7;
    ctx.lineWidth = Math.max(1, radius * 0.28);
    ctx.beginPath();
    ctx.arc(at.x, at.y, radius * 1.8, 0, Math.PI * 2);
    ctx.stroke();
    ctx.globalAlpha = 1;
  }
}

type TagRect = { x: number; y: number; width: number; height: number };

type TagChip = {
  rect: TagRect;
  color: string;
  label: string;
  anchorX: number;
  anchorY: number;
  up: boolean;
  displaced: boolean;
};

function overlaps(a: TagRect, b: TagRect): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

/** One resolved chip slot: where it went, which way it went and whether it moved. */
type TagSpot = { rect: TagRect; up: boolean; displaced: boolean };

/**
 * Where one chip lands: on its preferred side of the dot, stepped past whatever
 * is already placed, and clamped inside `visible` so a player holding an edge of
 * the map keeps their name instead of being drawn off it.
 *
 * `visible` is the plate's window onto the square, NOT the square: the two were
 * the same box until the plate started cropping, and clamping to the square let
 * a chip land in the padding the crop throws away, where the plate's border
 * sliced it in half.
 */
function placeTag(
  at: RadarPoint,
  width: number,
  height: number,
  radius: number,
  visible: RadarRect,
  prefersUp: boolean,
  placed: readonly TagRect[],
): TagSpot {
  const rawX = Math.round(at.x - width / 2);
  const x = Math.max(visible.x, Math.min(visible.x + visible.width - width, rawX));
  const candidate = (step: number, up: boolean): TagSpot => {
    const gap = radius * TAG_GAP + step * (height + TAG_STACK_GAP);
    const rawY = Math.round(up ? at.y - gap - height : at.y + gap);
    const y = Math.max(visible.y, Math.min(visible.y + visible.height - height, rawY));
    return { rect: { x, y, width, height }, up, displaced: step > 0 || x !== rawX || y !== rawY };
  };

  // Two searches, one step per chip already placed, and never a fixed cap. A cap
  // resolved every surplus chip to the SAME rectangle, so names stacked on one
  // spawn were painted exactly on top of each other and only the last of them
  // existed. Stepping alone is not enough either: a stack that reaches the
  // window's edge saturates against the clamp above, where one more step is
  // again the same rectangle — so once the preferred side is full the search
  // turns round and walks back past the dot, which is always empty by then.
  let last = candidate(0, prefersUp);
  for (const up of [prefersUp, !prefersUp]) {
    for (let step = 0; step <= placed.length; step += 1) {
      const spot = candidate(step, up);
      if (!placed.some((other) => overlaps(other, spot.rect))) return spot;
      last = spot;
    }
  }
  return last;
}

/**
 * Every alive player's identity, in one pass above every blip.
 *
 * Inside `drawPlayerBlip` a name was painted before the next player's cone, ring
 * backing and dot went down on top of it, which is what left four of five tags
 * half-eaten at spawn. Here the chip is opaque: --fg-1 glyphs on a --surface-3
 * plate read at 15.8:1 however many cones are stacked behind them, and the side
 * colour moves to a rail so the name never joins the cyan it is sitting on.
 */
function drawPlayerTags(ctx: CanvasRenderingContext2D, scene: RadarScene): void {
  const { geometry, size, style, view } = scene;
  if (size < TAG_MIN_SIZE) return;

  const font = Math.max(TAG_FONT_MIN, Math.round(size * TAG_FONT_RATIO));
  const height = Math.round(font * 1.5);
  const radius = blipRadius(size);
  const ring = radius * TAG_RING_EDGE;
  // The box a chip has to stay inside is what the plate SHOWS of the square.
  const visible: RadarRect = {
    x: view.x * size,
    y: view.y * size,
    width: view.width * size,
    height: view.height * size,
  };
  ctx.font = `${font}px ${style.fontFamily}`;
  ctx.textAlign = 'left';
  ctx.textBaseline = 'middle';

  const placed: TagRect[] = [];
  const chips: TagChip[] = [];
  for (const sample of scene.samples) {
    // The dead lose their name with their dot, which `drawPlayerBlip` used to
    // get for free from its own early return.
    if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) continue;
    const label = scene.labels.get(sample.slot);
    if (label === undefined) continue;

    const side = sampleSide(sample);
    const at = worldToRendered(geometry.calibration, sample.x, sample.y, size);
    const width = Math.round(ctx.measureText(label).width) + TAG_RAIL + TAG_PAD_X * 2;
    // CT above, T below: half the cross-side collisions never happen, and the
    // offset is a second, redundant read of the side.
    const prefersUp = side === TACTICAL_SIDES.ct;
    const room = prefersUp
      ? at.y - radius * TAG_GAP - height >= visible.y
      : at.y + radius * TAG_GAP + height <= visible.y + visible.height;
    // `placeTag` may still turn the stack round when that side fills up, so the
    // side it reports — not the one asked for — is what the leader line follows.
    const spot = placeTag(at, width, height, radius, visible, room ? prefersUp : !prefersUp, placed);
    placed.push(spot.rect);
    chips.push({
      rect: spot.rect,
      color: sideColor(style, side),
      label,
      anchorX: Math.round(at.x),
      anchorY: at.y,
      up: spot.up,
      displaced: spot.displaced,
    });
  }
  if (chips.length === 0) return;

  // Leaders first, so a plate always covers the hairline that reaches it.
  ctx.globalAlpha = TAG_LEADER_ALPHA;
  for (const chip of chips) {
    if (!chip.displaced) continue;
    const from = chip.up ? chip.rect.y + height : Math.round(chip.anchorY + ring);
    const to = chip.up ? Math.round(chip.anchorY - ring) : chip.rect.y;
    if (to <= from) continue;
    ctx.fillStyle = chip.color;
    ctx.fillRect(chip.anchorX, from, 1, to - from);
  }
  ctx.globalAlpha = 1;

  for (const chip of chips) {
    const { rect } = chip;
    ctx.fillStyle = style.plate;
    ctx.fillRect(rect.x, rect.y, rect.width, rect.height);
    ctx.fillStyle = chip.color;
    ctx.fillRect(rect.x, rect.y, TAG_RAIL, rect.height);
    ctx.fillStyle = style.text;
    ctx.fillText(chip.label, rect.x + TAG_RAIL + TAG_PAD_X, rect.y + rect.height / 2);
  }
}

/**
 * Paints one frame over an already-blitted background: the trails, then the
 * events that have happened, then the players, then every identity chip on top
 * of all of them — a name is only worth drawing if nothing lands on it after.
 */
export function drawTacticalScene(ctx: CanvasRenderingContext2D, scene: RadarScene): void {
  drawTrails(ctx, scene);
  drawEvents(ctx, scene);
  for (const sample of scene.samples) {
    if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) drawPlayerBlip(ctx, scene, sample);
  }
  for (const sample of scene.samples) {
    if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) drawPlayerBlip(ctx, scene, sample);
  }
  drawPlayerTags(ctx, scene);
}
