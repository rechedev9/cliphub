import type { TacticalGeometry } from '../api/tactical.ts';
import type { RadarRect } from '../tactical-transform.ts';
import { mapSilhouette } from './map-silhouette.ts';

/** Width of the whole radar in plate units; world points map into it with `worldToRendered`. */
export const RADAR_PLATE_SIZE = 100;

/** What an SVG plate of a map draws: the map's radar image when there is one, else the walkable silhouette. */
export type RadarPlateFrame = { viewBox: string; base: RadarRect; width: number; height: number; image: boolean; silhouette: string | null };

/** A radar image spans the whole radar square whatever its pixel size, so the plate is that square. */
export function radarPlateFrame(geometry: TacticalGeometry, hasRadar: boolean): RadarPlateFrame | null {
  if (hasRadar) {
    const size = RADAR_PLATE_SIZE;
    return { viewBox: `0 0 ${size} ${size}`, base: { x: 0, y: 0, width: size, height: size }, width: size, height: size, image: true, silhouette: null };
  }
  const silhouette = mapSilhouette(geometry, RADAR_PLATE_SIZE);
  if (silhouette === null) return null;
  const [x = 0, y = 0, width = RADAR_PLATE_SIZE, height = RADAR_PLATE_SIZE] = silhouette.viewBox.split(' ').map(Number);
  return { viewBox: silhouette.viewBox, base: { x, y, width, height }, width, height, image: false, silhouette: silhouette.path };
}
