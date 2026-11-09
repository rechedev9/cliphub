import { TACTICAL_EVENT_KINDS, TACTICAL_SAMPLE_FLAGS, TACTICAL_SIDES, hasSampleFlags } from '@/lib/api/tactical';
import type { RadarCalibration, RadarLevel, TacticalEventKind, TacticalSample, TacticalSide } from '@/lib/api/tactical';
import { floorAlpha } from '@/lib/analysis/floors';
import { equipmentIconUrl } from '@/lib/api/game-assets';
import type { EquipmentIconName } from '@/lib/api/game-assets';
import { layoutTags } from '@/lib/replay/tag-layout';
import type { TagRect } from '@/lib/replay/tag-layout';
import { bombVisual, grenadeArc, utilityVisuals } from '@/lib/replay/three/utility-visuals';
import type { UtilityVisual } from '@/lib/replay/three/utility-visuals';
import type { TrailPoint } from '@/lib/tactical-replay';
import type { TimelineEvent } from '@/lib/tactical-timeline';
import { worldToRendered, yawToRadarAngle } from '@/lib/tactical-transform';
import type { RadarPoint } from '@/lib/tactical-transform';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import { drawHitLabels, drawHitLines, drawSightPairs, drawViewWedges } from './radar-analysis';
import type { FrameAnalysis } from './radar-analysis';

/** World radii of the grenade effects, in CS2 units. */
const SMOKE_RADIUS = 144;
const FIRE_RADIUS = 120;
const HE_RADIUS = 220;
const FLASH_RADIUS = 90;
/** A dead player's cross fades to this over `DEAD_FADE_SECONDS`. */
const DEAD_REST_ALPHA = 0.3;
const DEAD_FADE_SECONDS = 6;
const TAG_FONT_PX = 12;
const TAG_HEIGHT = 18;
const TAG_RAIL = 2;
const TAG_PAD = 5;

/** One round's frame on the map, already filtered by the layers. */
export type RoundFrame = {
  calibration: RadarCalibration;
  /** The floor the plate shows; on a two-floor map, everything on the other one recedes. */
  level: RadarLevel;
  /** Side of the radar square in CSS pixels. */
  size: number;
  samples: readonly TacticalSample[];
  trails: ReadonlyMap<number, readonly TrailPoint[]>;
  events: readonly TimelineEvent[];
  nowSeconds: number;
  secondsOf: (tick: number) => number;
  /** Seconds since each dead player died, for the fading cross. */
  deadFor: ReadonlyMap<number, number>;
  labels: ReadonlyMap<number, string>;
  selectedSlot: number | null;
  utility: boolean;
  bomb: boolean;
  style: RadarStyle;
  /** Sight and hit lines; only the focused round carries them. */
  analysis: FrameAnalysis | null;
};

/** Where a drawn player sits in the square's CSS pixels, for hover and click. */
export type DrawnPlayer = { slot: number; x: number; y: number; side: TacticalSide; alive: boolean };

function sampleSide(sample: TacticalSample): TacticalSide {
  return hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.sideT) ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
}

function sideColor(style: RadarStyle, side: TacticalSide): string {
  return side === TACTICAL_SIDES.ct ? style.ct : style.t;
}

/** The disc radius at this zoom: grows with the map, within readable bounds. */
export function discRadius(size: number): number {
  return Math.min(11, Math.max(5.5, size * 0.0085));
}

function worldPx(calibration: RadarCalibration, size: number, units: number): number {
  return (units / calibration.scale) * (size / calibration.size);
}

function hpColor(style: RadarStyle, health: number): string {
  if (health > 50) return '#f2f4f6';
  if (health > 20) return style.map;
  return style.bomb;
}

function softDisc(ctx: CanvasRenderingContext2D, options: { at: RadarPoint; radius: number; rgb: string; alpha: number; edge: number }): void {
  const { at, radius, rgb, alpha, edge } = options;
  if (!(radius > 0)) return;
  const gradient = ctx.createRadialGradient(at.x, at.y, 0, at.x, at.y, radius);
  gradient.addColorStop(0, `rgba(${rgb}, ${alpha})`);
  gradient.addColorStop(edge, `rgba(${rgb}, ${alpha * 0.85})`);
  gradient.addColorStop(1, `rgba(${rgb}, 0)`);
  ctx.fillStyle = gradient;
  ctx.beginPath();
  ctx.arc(at.x, at.y, radius, 0, Math.PI * 2);
  ctx.fill();
}

/** Draws with everything faded when `z` is on the floor the plate is not showing. */
function onFloor(ctx: CanvasRenderingContext2D, frame: RoundFrame, z: number, draw: () => void): void {
  const alpha = floorAlpha(frame, z);
  if (alpha >= 1) {
    draw();
    return;
  }
  // A filter fades the whole drawing, whatever globalAlpha each stroke sets.
  ctx.filter = `opacity(${alpha})`;
  draw();
  ctx.filter = 'none';
}

/** Small deterministic offsets so a fire reads as flames, not a circle. */
const FLAME_LOBES: readonly [number, number, number][] = [
  [0, 0, 0.75],
  [0.45, 0.2, 0.55],
  [-0.4, 0.3, 0.5],
  [0.15, -0.45, 0.55],
  [-0.3, -0.35, 0.45],
  [0.5, -0.25, 0.4],
];

const GRENADE_ICON: Partial<Record<TacticalEventKind, EquipmentIconName>> = {
  [TACTICAL_EVENT_KINDS.smoke]: 'smokegrenade',
  [TACTICAL_EVENT_KINDS.flash]: 'flashbang',
  [TACTICAL_EVENT_KINDS.he]: 'hegrenade',
  [TACTICAL_EVENT_KINDS.molotov]: 'inferno',
  [TACTICAL_EVENT_KINDS.decoy]: 'decoy',
};

/** The game's grenade icons; the map preloads them so a badge shows the moment its grenade lands. */
const icons = new Map<EquipmentIconName, HTMLImageElement>();

function iconImage(name: EquipmentIconName): HTMLImageElement | null {
  let image = icons.get(name);
  if (image === undefined) {
    image = new Image();
    image.src = equipmentIconUrl(name);
    icons.set(name, image);
  }
  return image.complete && image.naturalWidth > 0 ? image : null;
}

/** Starts loading every grenade icon; resolves once they all settled, so the map can redraw. */
export function preloadGrenadeIcons(): Promise<void> {
  const loads = Object.values(GRENADE_ICON).map((name) => {
    iconImage(name);
    const image = icons.get(name);
    if (image === undefined || image.complete) return Promise.resolve();
    return image.decode().catch(() => undefined);
  });
  return Promise.all(loads).then(() => undefined);
}

/** A small dark badge with the grenade's icon where it went off. */
function grenadeBadge(ctx: CanvasRenderingContext2D, kind: TacticalEventKind, at: RadarPoint, alpha: number): void {
  const name = GRENADE_ICON[kind];
  const image = name === undefined ? null : iconImage(name);
  if (image === null) return;
  const height = 12;
  const width = Math.min(18, (image.naturalWidth / image.naturalHeight) * height);
  ctx.globalAlpha = alpha;
  ctx.fillStyle = 'rgba(11, 12, 14, 0.78)';
  ctx.beginPath();
  ctx.arc(at.x, at.y, 10, 0, Math.PI * 2);
  ctx.fill();
  ctx.drawImage(image, at.x - width / 2, at.y - height / 2, width, height);
  ctx.globalAlpha = 1;
}

function drawGrenade(ctx: CanvasRenderingContext2D, frame: RoundFrame, visual: UtilityVisual): void {
  const { calibration, size } = frame;
  const at = worldToRendered(calibration, visual.at.x, visual.at.y, size);
  if (visual.arc !== null && visual.from !== null) {
    const path = grenadeArc(visual.from, visual.at, 16).map((point) => worldToRendered(calibration, point.x, point.y, size));
    const reach = Math.max(1, Math.round(visual.arc.progress * (path.length - 1)));
    ctx.globalAlpha = 0.7 * visual.arc.fade;
    ctx.strokeStyle = '#e6e9ec';
    ctx.lineWidth = 1.25;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.moveTo(path[0].x, path[0].y);
    for (let i = 1; i <= reach; i += 1) ctx.lineTo(path[i].x, path[i].y);
    ctx.stroke();
    ctx.setLineDash([]);
    if (visual.arc.progress < 1) {
      const head = path[reach];
      ctx.globalAlpha = 1;
      ctx.fillStyle = '#ffffff';
      ctx.beginPath();
      ctx.arc(head.x, head.y, 2.5, 0, Math.PI * 2);
      ctx.fill();
    }
    ctx.globalAlpha = 1;
  }
  const effect = visual.effect;
  if (effect === null) return;
  drawEffect(ctx, frame, visual, at);
  grenadeBadge(ctx, visual.kind, at, Math.min(1, effect.strength * 1.5));
}

function drawEffect(ctx: CanvasRenderingContext2D, frame: RoundFrame, visual: UtilityVisual, at: RadarPoint): void {
  const { calibration, size } = frame;
  const effect = visual.effect;
  if (effect === null) return;
  if (visual.kind === TACTICAL_EVENT_KINDS.smoke) {
    const radius = worldPx(calibration, size, SMOKE_RADIUS) * effect.grow;
    softDisc(ctx, { at, radius, rgb: '214, 219, 224', alpha: 0.78 * effect.strength, edge: 0.72 });
    ctx.globalAlpha = 0.5 * effect.strength;
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 1;
    ctx.beginPath();
    ctx.arc(at.x, at.y, radius * 0.86, 0, Math.PI * 2);
    ctx.stroke();
    ctx.globalAlpha = 1;
  } else if (visual.kind === TACTICAL_EVENT_KINDS.molotov) {
    const radius = worldPx(calibration, size, FIRE_RADIUS) * effect.grow;
    for (const [dx, dy, scale] of FLAME_LOBES) {
      softDisc(ctx, { at: { x: at.x + dx * radius, y: at.y + dy * radius }, radius: radius * scale, rgb: '255, 104, 36', alpha: 0.55 * effect.strength, edge: 0.55 });
    }
    softDisc(ctx, { at, radius: radius * 0.45, rgb: '255, 196, 80', alpha: 0.5 * effect.strength, edge: 0.4 });
  } else if (visual.kind === TACTICAL_EVENT_KINDS.flash) {
    softDisc(ctx, { at, radius: worldPx(calibration, size, FLASH_RADIUS) * effect.grow, rgb: '255, 255, 255', alpha: 0.9 * effect.strength, edge: 0.2 });
  } else if (visual.kind === TACTICAL_EVENT_KINDS.he) {
    ctx.globalAlpha = effect.strength;
    ctx.strokeStyle = '#ff7a3d';
    ctx.lineWidth = 2.5;
    ctx.beginPath();
    ctx.arc(at.x, at.y, worldPx(calibration, size, HE_RADIUS) * effect.grow, 0, Math.PI * 2);
    ctx.stroke();
    softDisc(ctx, { at, radius: worldPx(calibration, size, HE_RADIUS * 0.4) * effect.grow, rgb: '255, 150, 70', alpha: 0.5 * effect.strength, edge: 0.3 });
    ctx.globalAlpha = 1;
  } else {
    ctx.globalAlpha = effect.strength;
    ctx.strokeStyle = '#c9ced4';
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    ctx.arc(at.x, at.y, 6, 0, Math.PI * 2);
    ctx.stroke();
    ctx.globalAlpha = 1;
  }
}

function drawBomb(ctx: CanvasRenderingContext2D, frame: RoundFrame): void {
  const carried = frame.samples.some((sample) => hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb));
  const bomb = bombVisual({ events: frame.events, nowSeconds: frame.nowSeconds, carried, secondsOf: frame.secondsOf });
  if (bomb === null) return;
  onFloor(ctx, frame, bomb.at.z, () => drawBombAt(ctx, frame, bomb));
}

function drawBombAt(ctx: CanvasRenderingContext2D, frame: RoundFrame, bomb: NonNullable<ReturnType<typeof bombVisual>>): void {
  const at = worldToRendered(frame.calibration, bomb.at.x, bomb.at.y, frame.size);
  if (bomb.state === 'exploded') {
    softDisc(ctx, { at, radius: worldPx(frame.calibration, frame.size, 500) * bomb.effect.grow, rgb: '255, 110, 50', alpha: 0.6 * bomb.effect.strength, edge: 0.5 });
    return;
  }
  let color = frame.style.bomb;
  if (bomb.state === 'defused') color = frame.style.defuse;
  else if (bomb.state === 'dropped') color = '#d8a24a';
  ctx.fillStyle = color;
  ctx.strokeStyle = '#0b0c0e';
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.roundRect(at.x - 7, at.y - 5, 14, 10, 2);
  ctx.fill();
  ctx.stroke();
  ctx.fillStyle = '#0b0c0e';
  ctx.fillRect(at.x - 4, at.y - 2, 8, 1.5);
  ctx.fillRect(at.x - 4, at.y + 1, 5, 1.5);
}

function drawTrails(ctx: CanvasRenderingContext2D, frame: RoundFrame): void {
  ctx.lineCap = 'round';
  ctx.lineJoin = 'round';
  ctx.lineWidth = 1.5;
  for (const sample of frame.samples) {
    const points = frame.trails.get(sample.slot);
    if (points === undefined || points.length < 2) continue;
    ctx.strokeStyle = sideColor(frame.style, sampleSide(sample));
    // A trail recedes with its player, by the floor the player stands on now.
    const strength = floorAlpha(frame, sample.z);
    for (let i = 1; i < points.length; i += 1) {
      const from = worldToRendered(frame.calibration, points[i - 1].x, points[i - 1].y, frame.size);
      const to = worldToRendered(frame.calibration, points[i].x, points[i].y, frame.size);
      ctx.globalAlpha = (0.05 + 0.35 * (i / (points.length - 1))) * strength;
      ctx.beginPath();
      ctx.moveTo(from.x, from.y);
      ctx.lineTo(to.x, to.y);
      ctx.stroke();
    }
  }
  ctx.globalAlpha = 1;
}

function drawDead(ctx: CanvasRenderingContext2D, frame: RoundFrame, sample: TacticalSample, at: RadarPoint): void {
  const age = frame.deadFor.get(sample.slot) ?? DEAD_FADE_SECONDS;
  const arm = discRadius(frame.size) * 0.75;
  ctx.globalAlpha = DEAD_REST_ALPHA + (1 - DEAD_REST_ALPHA) * Math.max(0, 1 - age / DEAD_FADE_SECONDS);
  ctx.lineCap = 'round';
  ctx.strokeStyle = '#0b0c0e';
  ctx.lineWidth = 4;
  for (const pass of [0, 1]) {
    if (pass === 1) {
      ctx.strokeStyle = sideColor(frame.style, sampleSide(sample));
      ctx.lineWidth = 2;
    }
    ctx.beginPath();
    ctx.moveTo(at.x - arm, at.y - arm);
    ctx.lineTo(at.x + arm, at.y + arm);
    ctx.moveTo(at.x + arm, at.y - arm);
    ctx.lineTo(at.x - arm, at.y + arm);
    ctx.stroke();
  }
  ctx.globalAlpha = 1;
}

function drawAlive(ctx: CanvasRenderingContext2D, frame: RoundFrame, sample: TacticalSample, at: RadarPoint): void {
  const { style } = frame;
  const side = sampleSide(sample);
  const color = sideColor(style, side);
  const radius = discRadius(frame.size);
  const heading = (yawToRadarAngle(sample.yaw) * Math.PI) / 180;
  if (frame.analysis?.sight?.wedges.has(sample.slot) !== true) {
    const reach = radius * 6;
    const cone = ctx.createRadialGradient(at.x, at.y, radius, at.x, at.y, reach);
    cone.addColorStop(0, color);
    cone.addColorStop(1, 'rgba(0, 0, 0, 0)');
    ctx.globalAlpha = 0.42;
    ctx.fillStyle = cone;
    ctx.beginPath();
    ctx.moveTo(at.x, at.y);
    ctx.arc(at.x, at.y, reach, heading - 0.4, heading + 0.4);
    ctx.closePath();
    ctx.fill();
    ctx.globalAlpha = 1;
  }

  if (frame.selectedSlot === sample.slot) {
    ctx.strokeStyle = '#ffffff';
    ctx.lineWidth = 2;
    ctx.beginPath();
    ctx.arc(at.x, at.y, radius + 7, 0, Math.PI * 2);
    ctx.stroke();
  }
  const health = Math.max(0, Math.min(100, sample.health));
  const ring = radius + 2.75;
  ctx.lineWidth = 2.5;
  ctx.strokeStyle = 'rgba(11, 12, 14, 0.85)';
  ctx.beginPath();
  ctx.arc(at.x, at.y, ring, 0, Math.PI * 2);
  ctx.stroke();
  if (health > 0) {
    ctx.strokeStyle = hpColor(style, health);
    ctx.beginPath();
    ctx.arc(at.x, at.y, ring, -Math.PI / 2, -Math.PI / 2 + (health / 100) * Math.PI * 2);
    ctx.stroke();
  }
  ctx.fillStyle = color;
  ctx.strokeStyle = '#ffffff';
  ctx.lineWidth = 1.5;
  ctx.beginPath();
  ctx.arc(at.x, at.y, radius, 0, Math.PI * 2);
  ctx.fill();
  ctx.stroke();

  const flashed = (sample.flashed ?? 0) > 0 || hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.blinded);
  if (flashed) {
    ctx.fillStyle = 'rgba(255, 255, 255, 0.8)';
    ctx.beginPath();
    ctx.arc(at.x, at.y, radius * 0.6, 0, Math.PI * 2);
    ctx.fill();
  }
  if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.hasBomb)) {
    const s = Math.max(6, radius * 0.95);
    ctx.fillStyle = style.bomb;
    ctx.strokeStyle = '#0b0c0e';
    ctx.lineWidth = 1.25;
    ctx.beginPath();
    ctx.roundRect(at.x + radius * 0.45, at.y + radius * 0.45, s, s * 0.75, 1.5);
    ctx.fill();
    ctx.stroke();
  }
  if (hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.defusing)) {
    ctx.strokeStyle = style.defuse;
    ctx.lineWidth = 2;
    ctx.setLineDash([3, 3]);
    ctx.beginPath();
    ctx.arc(at.x, at.y, radius + 7, 0, Math.PI * 2);
    ctx.stroke();
    ctx.setLineDash([]);
  }
}

/** Name tags last, above every disc, laid out so none covers another or a disc. */
function drawTags(ctx: CanvasRenderingContext2D, options: { frame: RoundFrame; players: readonly DrawnPlayer[]; visible: TagRect; spots: Map<number, number> }): void {
  const { frame, players, visible, spots } = options;
  if (frame.labels.size === 0) return;
  ctx.font = `600 ${TAG_FONT_PX}px ${frame.style.fontFamily}`;
  // A player panned off the canvas gets no tag, or it would sit at the edge far from anyone.
  const inside = (player: DrawnPlayer): boolean => player.x >= visible.x && player.x <= visible.x + visible.width && player.y >= visible.y && player.y <= visible.y + visible.height;
  const alive = players.filter((player) => player.alive && frame.labels.has(player.slot) && inside(player));
  const requests = alive.map((player) => {
    const label = frame.labels.get(player.slot) ?? '';
    return { id: player.slot, x: player.x, y: player.y, width: Math.ceil(ctx.measureText(label).width) + TAG_RAIL + TAG_PAD * 2, height: TAG_HEIGHT };
  });
  const placed = layoutTags({ tags: requests, discs: alive, radius: discRadius(frame.size) + 3, bounds: visible, prefer: spots });
  spots.clear();
  ctx.textAlign = 'left';
  ctx.textBaseline = 'middle';
  for (const tag of placed) {
    spots.set(tag.id, tag.spot);
    const player = alive.find((entry) => entry.slot === tag.id);
    if (player === undefined) continue;
    const { rect } = tag;
    const color = sideColor(frame.style, player.side);
    if (tag.displaced) {
      ctx.strokeStyle = color;
      ctx.globalAlpha = 0.6;
      ctx.lineWidth = 1;
      ctx.beginPath();
      ctx.moveTo(player.x, player.y);
      ctx.lineTo(Math.min(Math.max(player.x, rect.x), rect.x + rect.width), Math.min(Math.max(player.y, rect.y), rect.y + rect.height));
      ctx.stroke();
      ctx.globalAlpha = 1;
    }
    ctx.fillStyle = frame.selectedSlot === tag.id ? 'rgba(46, 50, 56, 0.95)' : 'rgba(11, 12, 14, 0.82)';
    ctx.beginPath();
    ctx.roundRect(rect.x, rect.y, rect.width, rect.height, 3);
    ctx.fill();
    ctx.fillStyle = color;
    ctx.fillRect(rect.x, rect.y + 3, TAG_RAIL, rect.height - 6);
    ctx.fillStyle = '#f2f4f6';
    ctx.fillText(frame.labels.get(tag.id) ?? '', rect.x + TAG_RAIL + TAG_PAD, rect.y + rect.height / 2 + 0.5);
  }
}

/**
 * Paints one round over the baked plate: smokes, fires and throws first, the
 * view wedges and trails, the bomb on the ground, the dead, sight and hit
 * lines, the living, and the name tags on top. Answers where each player was
 * drawn, for hover and click.
 */
export function drawRoundFrame(ctx: CanvasRenderingContext2D, frame: RoundFrame, tags: { visible: TagRect; spots: Map<number, number> } | null): DrawnPlayer[] {
  if (frame.utility) {
    for (const visual of utilityVisuals(frame.events, frame.nowSeconds, frame.secondsOf)) onFloor(ctx, frame, visual.at.z, () => drawGrenade(ctx, frame, visual));
  }
  const canvas = { ctx, calibration: frame.calibration, size: frame.size, style: frame.style, samples: frame.samples };
  const sight = frame.analysis?.sight ?? null;
  const hits = frame.analysis?.hits ?? [];
  if (sight !== null) drawViewWedges(canvas, sight);
  drawTrails(ctx, frame);
  if (frame.bomb) drawBomb(ctx, frame);
  const players: DrawnPlayer[] = [];
  for (const sample of frame.samples) {
    const at = worldToRendered(frame.calibration, sample.x, sample.y, frame.size);
    const alive = hasSampleFlags(sample.flags, TACTICAL_SAMPLE_FLAGS.alive);
    players.push({ slot: sample.slot, x: at.x, y: at.y, side: sampleSide(sample), alive });
    if (!alive) onFloor(ctx, frame, sample.z, () => drawDead(ctx, frame, sample, at));
  }
  if (sight !== null) drawSightPairs(canvas, sight);
  drawHitLines(canvas, hits);
  for (const sample of frame.samples) {
    const player = players.find((entry) => entry.slot === sample.slot);
    if (player !== undefined && player.alive) onFloor(ctx, frame, sample.z, () => drawAlive(ctx, frame, sample, player));
  }
  drawHitLabels(canvas, hits);
  if (tags !== null) drawTags(ctx, { frame, players, visible: tags.visible, spots: tags.spots });
  return players;
}
