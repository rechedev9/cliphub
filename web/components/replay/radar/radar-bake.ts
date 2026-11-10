import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import { NAV_RADAR_PALETTE, floorPolygons, heightTone, placeTints, toneColor, toneRange } from '@/lib/replay/nav-radar';
import type { NavPolygon, NavRadarModel } from '@/lib/replay/nav-radar';
import { placeLabel } from '@/lib/tactical-labels';
import { worldToRendered } from '@/lib/tactical-transform';
import type { RadarRect } from '@/lib/tactical-transform';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';
import type { RealPlate } from './real-plate';

/** What the map plate is painted from. */
export type MapArt =
  | { kind: 'nav'; model: NavRadarModel }
  | { kind: 'image'; upper: HTMLImageElement; lower: HTMLImageElement | null }
  | { kind: 'real'; plate: RealPlate }
  | { kind: 'grid' };

/** Floors this many world units apart get an edge between them, which reads as a step. */
const BAND_UNITS = 40;
const OUTLINE_PX = 2.4;
const SEAM_PX = 0.9;
const SITE_COLOR = '#c9ced4';

function tracePolygon(ctx: CanvasRenderingContext2D, polygon: NavPolygon): void {
  const { points } = polygon;
  ctx.moveTo(points[0], points[1]);
  for (let i = 2; i < points.length; i += 2) ctx.lineTo(points[i], points[i + 1]);
  ctx.closePath();
}

/**
 * The nav areas in world coordinates under a world-to-device transform: per
 * height band, a dark stroke round every area, then the fills on top. Inside a
 * band the fills cover the strokes, so only the outer walls and the steps
 * between bands keep a crisp edge. Bombsites and the CT spawn take their tint.
 */
function paintFloors(ctx: CanvasRenderingContext2D, options: { polygons: readonly NavPolygon[]; tints: ReadonlyMap<NavPolygon, string>; unitsPerPixel: number }): void {
  const { polygons, tints, unitsPerPixel } = options;
  const range = toneRange(polygons);
  let start = 0;
  while (start < polygons.length) {
    const band = Math.floor(polygons[start].z / BAND_UNITS);
    let end = start;
    while (end < polygons.length && Math.floor(polygons[end].z / BAND_UNITS) === band) end += 1;
    ctx.beginPath();
    for (let i = start; i < end; i += 1) tracePolygon(ctx, polygons[i]);
    ctx.strokeStyle = NAV_RADAR_PALETTE.outline;
    ctx.lineWidth = OUTLINE_PX * unitsPerPixel;
    ctx.lineJoin = 'round';
    ctx.stroke();
    ctx.lineWidth = SEAM_PX * unitsPerPixel;
    for (let i = start; i < end; i += 1) {
      const color = tints.get(polygons[i]) ?? toneColor(heightTone(polygons[i].z, range.minZ, range.maxZ), NAV_RADAR_PALETTE.low, NAV_RADAR_PALETTE.high);
      ctx.beginPath();
      tracePolygon(ctx, polygons[i]);
      ctx.fillStyle = color;
      ctx.strokeStyle = color;
      ctx.fill();
      ctx.stroke();
    }
    start = end;
  }
}

/** Only the floor on show is painted, in the same greys on either floor: nothing of the other one ghosts through. */
function paintNav(ctx: CanvasRenderingContext2D, options: { model: NavRadarModel; geometry: TacticalGeometry; level: RadarLevel; size: number; dpr: number; view: RadarRect }): void {
  const { model, geometry, level, size, dpr, view } = options;
  const { calibration } = geometry;
  const k = size / calibration.size / calibration.scale;
  // World x,y straight to device pixels of the window: one transform for every vertex.
  ctx.setTransform(k * dpr, 0, 0, -k * dpr, (-calibration.pos_x * k - view.x * size) * dpr, (calibration.pos_y * k - view.y * size) * dpr);
  const split = calibration.lower_altitude_max;
  paintFloors(ctx, { polygons: floorPolygons(model, split, level), tints: placeTints(model, geometry.callouts, split), unitsPerPixel: 1 / k });
}

function siteBadge(ctx: CanvasRenderingContext2D, x: number, y: number, letter: string, font: string): void {
  const half = 11;
  ctx.fillStyle = SITE_COLOR;
  ctx.beginPath();
  ctx.roundRect(x - half, y - half, half * 2, half * 2, 4);
  ctx.fill();
  ctx.strokeStyle = 'rgba(0, 0, 0, 0.55)';
  ctx.lineWidth = 1.5;
  ctx.stroke();
  ctx.fillStyle = '#0b0c0e';
  ctx.font = `700 14px ${font}`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(letter, x, y + 0.5);
}

function spawnBadge(ctx: CanvasRenderingContext2D, options: { x: number; y: number; label: string; color: string; font: string }): void {
  const { x, y, label, color, font } = options;
  ctx.font = `700 12px ${font}`;
  const width = ctx.measureText(label).width + 10;
  ctx.fillStyle = 'rgba(11, 12, 14, 0.82)';
  ctx.beginPath();
  ctx.roundRect(x - width / 2, y - 9, width, 18, 9);
  ctx.fill();
  ctx.strokeStyle = color;
  ctx.lineWidth = 1.5;
  ctx.stroke();
  ctx.fillStyle = color;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.fillText(label, x, y + 0.5);
}

/** The name of a zone, small and light with a dark edge so it reads on any grey. */
function zoneName(ctx: CanvasRenderingContext2D, x: number, y: number, name: string, font: string): void {
  ctx.font = `600 11px ${font}`;
  ctx.textAlign = 'center';
  ctx.textBaseline = 'middle';
  ctx.lineWidth = 3;
  ctx.strokeStyle = 'rgba(11, 12, 14, 0.9)';
  ctx.strokeText(name, x, y);
  ctx.fillStyle = '#d9dde1';
  ctx.fillText(name, x, y);
}

/**
 * Bombsite and spawn badges and the names of the places the demo recorded: on the
 * flat map only with Nombres de zonas on, since it carries no text otherwise. The
 * official overview already draws its own, and a two-floor map names each
 * floor's places on that floor's plate.
 */
function paintMarkers(ctx: CanvasRenderingContext2D, options: { geometry: TacticalGeometry; level: RadarLevel; size: number; style: RadarStyle; kind: MapArt['kind']; zoneNames: boolean }): void {
  const { geometry, level, size, style, kind, zoneNames } = options;
  // The real map keeps its badges as before; the flat plate carries none until Nombres de zonas is on.
  if (kind === 'image' || (!zoneNames && kind !== 'real')) return;
  const split = geometry.calibration.lower_altitude_max !== undefined;
  for (const callout of geometry.callouts) {
    if (split && callout.level !== level) continue;
    const at = worldToRendered(geometry.calibration, callout.center[0], callout.center[1], size);
    const name = callout.name.toLowerCase().replace(/[^a-z]/g, '');
    if (name === 'bombsitea') siteBadge(ctx, at.x, at.y, 'A', style.fontFamily);
    else if (name === 'bombsiteb') siteBadge(ctx, at.x, at.y, 'B', style.fontFamily);
    else if (name === 'tspawn') spawnBadge(ctx, { ...at, label: 'T', color: style.t, font: style.fontFamily });
    else if (name === 'ctspawn') spawnBadge(ctx, { ...at, label: 'CT', color: style.ct, font: style.fontFamily });
    else if (zoneNames) zoneName(ctx, at.x, at.y, placeLabel(callout.name), style.fontFamily);
  }
}

/**
 * Bakes the visible window of the map plate at device resolution: the art,
 * then the zone names when asked. Everything draws in the radar
 * square's coordinates (`worldToRendered`), so the players land on it exactly.
 */
export function bakeMapPlate(options: {
  art: MapArt;
  geometry: TacticalGeometry;
  level: RadarLevel;
  size: number;
  dpr: number;
  view: RadarRect;
  style: RadarStyle;
  /** Bombsite and spawn badges and zone names on the schematic plate. */
  zoneNames?: boolean;
}): HTMLCanvasElement {
  const { art, geometry, level, size, dpr, view, style } = options;
  const zoneNames = options.zoneNames ?? false;
  if (art.kind === 'grid') return renderRadarBackground(geometry, level, size, dpr, style, view);
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(view.width * size * dpr));
  canvas.height = Math.max(1, Math.round(view.height * size * dpr));
  const ctx = canvas.getContext('2d');
  if (ctx === null) return canvas;
  ctx.fillStyle = NAV_RADAR_PALETTE.void;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  if (art.kind === 'nav') paintNav(ctx, { model: art.model, geometry, level, size, dpr, view });
  if (art.kind === 'real') art.plate.draw(ctx, { calibration: geometry.calibration, level, size, dpr, view });
  ctx.setTransform(dpr, 0, 0, dpr, -view.x * size * dpr, -view.y * size * dpr);
  if (art.kind === 'image') {
    // The game's overview is the radar square itself, so it lands on the calibration exactly.
    const image = level === 'lower' && art.lower !== null ? art.lower : art.upper;
    ctx.imageSmoothingQuality = 'high';
    ctx.drawImage(image, 0, 0, size, size);
  }
  paintMarkers(ctx, { geometry, level, size, style, kind: art.kind, zoneNames });
  return canvas;
}
