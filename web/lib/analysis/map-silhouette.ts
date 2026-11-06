import { RADAR_LEVELS, type TacticalGeometry } from '../api/tactical.ts';
import { cellRadarRect, radarViewRect } from '../tactical-transform.ts';

/**
 * A map's walkable space as one SVG path: the same play-derived occupancy cells
 * the 2D radar bakes, on the default level, framed by the radar's own view
 * window. Small enough for a map card; `null` when the geometry has no cells.
 */
export function mapSilhouette(geometry: TacticalGeometry, size = 100): { path: string; viewBox: string } | null {
  const level = geometry.levels.find((entry) => entry.name === RADAR_LEVELS.default) ?? geometry.levels[0];
  if (level === undefined || level.cells.length === 0 || !(geometry.cell_size > 0)) return null;
  const view = radarViewRect(geometry);
  const r = (n: number): number => Math.round(n * 100) / 100;
  const parts: string[] = [];
  for (const cell of level.cells) {
    if (!(cell[2] > 0)) continue;
    const rect = cellRadarRect(geometry, cell, size);
    if (!(rect.width > 0) || !(rect.height > 0)) continue;
    parts.push(`M${r(rect.x)} ${r(rect.y)}h${r(rect.width)}v${r(rect.height)}h${r(-rect.width)}z`);
  }
  if (parts.length === 0) return null;
  return {
    path: parts.join(''),
    viewBox: `${r(view.x * size)} ${r(view.y * size)} ${r(view.width * size)} ${r(view.height * size)}`,
  };
}
