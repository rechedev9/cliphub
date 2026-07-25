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
 * offscreen bitmap; the round is what changes 60 times a second.
 */

/** Colours and type, resolved once from the design tokens on the live element. */
export type RadarStyle = {
  background: string;
  panel: string;
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
  /** Side of the square radar, in CSS pixels. */
  size: number;
  geometry: TacticalGeometry;
  /** Level drawn at full strength; the others are dimmed in the background pass. */
  activeLevel: RadarLevel;
  samples: readonly TacticalSample[];
  trails: ReadonlyMap<number, readonly TrailPoint[]>;
  /** Events that have already happened at the playhead, oldest first. */
  events: readonly TimelineEvent[];
  /** Playhead position in bar seconds, used to fade older marks. */
  nowSeconds: number;
  /** Short display name per slot, drawn under the dot. */
  labels: ReadonlyMap<number, string>;
  style: RadarStyle;
};

/** Occupancy weight is long-tailed, so the ramp is on its square root. */
const CELL_MIN_ALPHA = 0.1;
const CELL_ALPHA_RANGE = 0.5;
/** Levels the player is not on stay legible but recede. */
const INACTIVE_LEVEL_ALPHA = 0.22;
/** Callouts below this share of the busiest one would only add noise. */
const CALLOUT_SAMPLE_SHARE = 0.06;
/** Below this size a callout label costs more legibility than it adds. */
const CALLOUT_MIN_SIZE = 380;
/** An event mark fades to its resting alpha over this many seconds. */
const EVENT_FRESH_SECONDS = 4;

function sideColor(style: RadarStyle, side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? style.ct : style.t;
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

  ctx.font = `${Math.round(size * 0.018)}px ${style.fontFamily}`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillStyle = style.callout;
  for (const callout of geometry.callouts) {
    if (callout.samples / busiest < CALLOUT_SAMPLE_SHARE) continue;
    if (callout.level !== activeLevel) continue;
    const point = worldToRendered(geometry.calibration, callout.center[0], callout.center[1], size);
    ctx.globalAlpha = 0.55;
    ctx.fillText(callout.name.toUpperCase(), point.x, point.y);
  }
  ctx.globalAlpha = 1;
}

/**
 * Bakes the play-derived map: every level's occupancy grid, the active one at
 * full strength, plus the callout labels. The bitmap is device-resolution, so
 * the caller can blit it into a CSS-pixel box and keep the text crisp.
 */
export function renderRadarBackground(
  geometry: TacticalGeometry,
  activeLevel: RadarLevel,
  size: number,
  dpr: number,
  style: RadarStyle,
): HTMLCanvasElement {
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(size * dpr));
  canvas.height = canvas.width;
  const ctx = canvas.getContext('2d');
  if (ctx === null) return canvas;
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0);

  ctx.fillStyle = style.panel;
  ctx.fillRect(0, 0, size, size);

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
    const side = hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT)
      ? TACTICAL_SIDES.t
      : TACTICAL_SIDES.ct;
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

function drawPlayer(ctx: CanvasRenderingContext2D, scene: RadarScene, sample: TacticalSample): void {
  const { geometry, size, style } = scene;
  const at = worldToRendered(geometry.calibration, sample.x, sample.y, size);
  const side = hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT)
    ? TACTICAL_SIDES.t
    : TACTICAL_SIDES.ct;
  const color = sideColor(style, side);
  const radius = Math.max(3.5, size * 0.0105);

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
  const reach = radius * 5.2;
  ctx.globalAlpha = 0.16;
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

  const label = scene.labels.get(sample.slot);
  if (label !== undefined && size >= CALLOUT_MIN_SIZE) {
    ctx.font = `${Math.round(size * 0.017)}px ${style.fontFamily}`;
    ctx.textAlign = 'center';
    ctx.textBaseline = 'top';
    ctx.lineWidth = Math.max(2, size * 0.005);
    ctx.strokeStyle = style.panel;
    ctx.lineJoin = 'round';
    // A halo in the panel colour keeps the name legible over any occupancy tone.
    ctx.strokeText(label, at.x, at.y + radius * 2.1);
    ctx.fillStyle = style.text;
    ctx.globalAlpha = 0.85;
    ctx.fillText(label, at.x, at.y + radius * 2.1);
    ctx.globalAlpha = 1;
  }
}

/**
 * Paints one frame over an already-blitted background: the trails, then the
 * events that have happened, then the players on top.
 */
export function drawTacticalScene(ctx: CanvasRenderingContext2D, scene: RadarScene): void {
  drawTrails(ctx, scene);
  drawEvents(ctx, scene);
  for (const sample of scene.samples) {
    if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) drawPlayer(ctx, scene, sample);
  }
  for (const sample of scene.samples) {
    if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) drawPlayer(ctx, scene, sample);
  }
}
