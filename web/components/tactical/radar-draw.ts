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
import type { TacticalDrawCache } from '@/lib/tactical-draw-cache';


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
  /** Sorted timeline. Drawing stops at the first future event. */
  events: readonly TimelineEvent[];
  /** Playhead position in bar seconds, used to fade older marks. */
  nowSeconds: number;
  /** Short display name per slot, drawn on a chip in a pass above every blip. */
  labels: ReadonlyMap<number, string>;
  style: RadarStyle;
};

const CELL_MIN_ALPHA = 0.07;
const CELL_ALPHA_RANGE = 0.38;
const INACTIVE_LEVEL_ALPHA = 0.22;
const CALLOUT_SAMPLE_SHARE = 0.12;
const CALLOUT_MIN_SIZE = 380;
const EVENT_FRESH_SECONDS = 4;

const TAG_MIN_SIZE = 380;
const TAG_FONT_MIN = 12;
const TAG_FONT_RATIO = 0.016;
const TAG_PAD_X = 4;
const TAG_RAIL = 2;
const TAG_GAP = 2.6;
const TAG_STACK_GAP = 3;
const TAG_RING_EDGE = 1.6;
const TAG_LEADER_ALPHA = 0.5;

function sideColor(style: RadarStyle, side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? style.ct : style.t;
}

function sampleSide(sample: TacticalSample): TacticalSide {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT)
    ? TACTICAL_SIDES.t
    : TACTICAL_SIDES.ct;
}

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
  ctx.translate(-view.x * size, -view.y * size);

  for (const level of geometry.levels) {
    if (level.name === activeLevel) continue;
    drawLevelCells(ctx, geometry, level, size, style, INACTIVE_LEVEL_ALPHA);
  }
  const active = geometry.levels.find((level) => level.name === activeLevel);
  if (active !== undefined) drawLevelCells(ctx, geometry, active, size, style, 1);
  else {
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

function drawEvents(ctx: CanvasRenderingContext2D, scene: RadarScene, cache?: TacticalDrawCache): void {
  const { geometry, size, style } = scene;
  const mark = Math.max(3, size * 0.009);

  for (const entry of scene.events) {
    if (entry.seconds > scene.nowSeconds) break;
    const { event } = entry;
    const age = scene.nowSeconds - entry.seconds;
    const freshness = 1 - Math.min(1, Math.max(0, age) / EVENT_FRESH_SECONDS);
    const alpha = 0.32 + 0.55 * freshness;
    const points = cache?.eventPoints(geometry, size, entry);
    const at = points?.at ?? worldToRendered(geometry.calibration, event.pos[0], event.pos[1], size);
    ctx.lineWidth = Math.max(1.25, size * 0.003);

    if (event.kind === TACTICAL_EVENT_KINDS.kill) {
      const victimSide = event.side === undefined ? undefined : opponentSide(event.side);
      const victim = points?.target ?? worldToRendered(geometry.calibration, event.target_pos[0], event.target_pos[1], size);
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

  const heading = (yawToRadarAngle(sample.yaw) * Math.PI) / 180;
  const half = (26 * Math.PI) / 180;
  const reach = radius * 4.4;
  ctx.globalAlpha = 0.11;
  ctx.fillStyle = color;
  ctx.beginPath();
  ctx.moveTo(at.x, at.y);
  ctx.arc(at.x, at.y, reach, heading - half, heading + half);
  ctx.closePath();
  ctx.fill();

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

function drawPlayerTags(ctx: CanvasRenderingContext2D, scene: RadarScene, cache?: TacticalDrawCache): void {
  const { geometry, size, style, view } = scene;
  if (size < TAG_MIN_SIZE) return;

  const font = Math.max(TAG_FONT_MIN, Math.round(size * TAG_FONT_RATIO));
  const height = Math.round(font * 1.5);
  const radius = blipRadius(size);
  const ring = radius * TAG_RING_EDGE;
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
    if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) continue;
    const label = scene.labels.get(sample.slot);
    if (label === undefined) continue;

    const side = sampleSide(sample);
    const at = worldToRendered(geometry.calibration, sample.x, sample.y, size);
    const measured = cache?.labelWidth(ctx.font, label, () => ctx.measureText(label).width)
      ?? ctx.measureText(label).width;
    const width = Math.round(measured) + TAG_RAIL + TAG_PAD_X * 2;
    const prefersUp = side === TACTICAL_SIDES.ct;
    const room = prefersUp
      ? at.y - radius * TAG_GAP - height >= visible.y
      : at.y + radius * TAG_GAP + height <= visible.y + visible.height;
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

export function drawTacticalScene(ctx: CanvasRenderingContext2D, scene: RadarScene, cache?: TacticalDrawCache): void {
  drawTrails(ctx, scene);
  drawEvents(ctx, scene, cache);
  for (const sample of scene.samples) {
    if (!hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) drawPlayerBlip(ctx, scene, sample);
  }
  for (const sample of scene.samples) {
    if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive)) drawPlayerBlip(ctx, scene, sample);
  }
  drawPlayerTags(ctx, scene, cache);
}
