import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import { NAV_GROW, NAV_RADAR_PALETTE, floorPolygons, heightTone, placeTints, toneColor, toneRange } from '@/lib/replay/nav-radar';
import type { NavPolygon, NavRadarModel } from '@/lib/replay/nav-radar';
import type { RadarRect } from '@/lib/tactical-transform';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';

/** What the map plate is painted from. */
export type MapArt =
  | { kind: 'nav'; model: NavRadarModel }
  | { kind: 'grid' };

/** Floors this many world units apart get an edge between them, which reads as a step. */
const BAND_UNITS = 40;
const OUTLINE_PX = 2.4;

function tracePolygon(ctx: CanvasRenderingContext2D, polygon: NavPolygon): void {
  const { points } = polygon;
  ctx.moveTo(points[0], points[1]);
  for (let i = 2; i < points.length; i += 2) ctx.lineTo(points[i], points[i + 1]);
  ctx.closePath();
}

/**
 * The nav areas in world coordinates under a world-to-device transform: per
 * height band, a dark stroke round every area, then the fills on top, each
 * grown by `NAV_GROW` with round corners so the ground reaches the walls and
 * closes the gaps between areas. Inside a band the fills cover the strokes, so
 * only the outer walls and the steps between bands keep a crisp edge.
 * Bombsites and the CT spawn take their tint.
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
    ctx.lineWidth = 2 * NAV_GROW + OUTLINE_PX * unitsPerPixel;
    ctx.lineJoin = 'round';
    ctx.stroke();
    ctx.lineWidth = 2 * NAV_GROW;
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

/**
 * Bakes the visible window of the map plate at device resolution. Everything
 * draws in the radar square's coordinates (`worldToRendered`), so the players land on it exactly.
 */
export function bakeMapPlate(options: {
  art: MapArt;
  geometry: TacticalGeometry;
  level: RadarLevel;
  size: number;
  dpr: number;
  view: RadarRect;
  style: RadarStyle;
}): HTMLCanvasElement {
  const { art, geometry, level, size, dpr, view, style } = options;
  if (art.kind === 'grid') return renderRadarBackground(geometry, level, size, dpr, style, view);
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(view.width * size * dpr));
  canvas.height = Math.max(1, Math.round(view.height * size * dpr));
  const ctx = canvas.getContext('2d');
  if (ctx === null) return canvas;
  ctx.fillStyle = NAV_RADAR_PALETTE.void;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  if (art.kind === 'nav') paintNav(ctx, { model: art.model, geometry, level, size, dpr, view });
  return canvas;
}
