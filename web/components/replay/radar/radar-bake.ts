import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import { NAV_RADAR_PALETTE, floorPolygons, placeRegions } from '@/lib/replay/nav-radar';
import type { NavRadarModel } from '@/lib/replay/nav-radar';
import { paintPlate } from '@/components/replay/radar/plate-paint';
import type { RadarRect } from '@/lib/tactical-transform';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';

/** What the map plate is painted from. */
export type MapArt =
  | { kind: 'nav'; model: NavRadarModel }
  | { kind: 'grid' };

/** Only the floor on show is painted, in the same greys on either floor: nothing of the other one ghosts through. */
function paintNav(ctx: CanvasRenderingContext2D, options: { model: NavRadarModel; geometry: TacticalGeometry; level: RadarLevel; size: number; dpr: number; view: RadarRect }): void {
  const { model, geometry, level, size, dpr, view } = options;
  const { calibration } = geometry;
  const k = size / calibration.size / calibration.scale;
  const split = calibration.lower_altitude_max;
  const regions = placeRegions(model, geometry.callouts, split).filter((region) => split === undefined || region.level === level);
  // World x,y straight to device pixels of the window: one matrix for every vertex.
  const matrix = { scale: k * dpr, x: (-calibration.pos_x * k - view.x * size) * dpr, y: (calibration.pos_y * k - view.y * size) * dpr };
  paintPlate({ target: ctx, width: ctx.canvas.width, height: ctx.canvas.height, polygons: floorPolygons(model, split, level), regions, matrix });
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
