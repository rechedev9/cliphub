import { RADAR_LEVELS } from '@/lib/api/tactical';
import type { RadarLevel, TacticalGeometry } from '@/lib/api/tactical';
import type { RadarRect } from '@/lib/tactical-transform';
import { radarImageFits } from '@/lib/replay/map-view';
import { renderRadarBackground } from '@/components/tactical/radar-draw';
import type { RadarStyle } from '@/components/tactical/radar-draw';

/** The radar image of each floor; a one-floor map has no lower one. */
export type RadarImages = { default: HTMLImageElement; lower: HTMLImageElement | null };

/** What the map plate is painted from. */
export type MapArt = { kind: 'radar'; images: RadarImages } | { kind: 'grid' };

/**
 * Bakes the visible window of the map plate at device resolution. Everything
 * draws in the radar square's coordinates (`worldToRendered`), so the players land on it exactly:
 * every radar image spans that square, whatever its pixel size.
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
  // A derived calibration frames this demo's positions, not the radar's square.
  const images = art.kind === 'radar' && radarImageFits(geometry.calibration) ? art.images : null;
  const image = level === RADAR_LEVELS.lower ? (images?.lower ?? null) : (images?.default ?? null);
  if (image === null) return renderRadarBackground(geometry, level, size, dpr, style, view);
  const canvas = document.createElement('canvas');
  canvas.width = Math.max(1, Math.round(view.width * size * dpr));
  canvas.height = Math.max(1, Math.round(view.height * size * dpr));
  const ctx = canvas.getContext('2d');
  if (ctx === null) return canvas;
  ctx.fillStyle = style.background;
  ctx.fillRect(0, 0, canvas.width, canvas.height);
  ctx.imageSmoothingQuality = 'high';
  ctx.drawImage(image, -view.x * size * dpr, -view.y * size * dpr, size * dpr, size * dpr);
  return canvas;
}
