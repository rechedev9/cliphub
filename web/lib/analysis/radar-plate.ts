import type { TacticalGeometry } from '../api/tactical.ts';
import type { RadarRect } from '../tactical-transform.ts';
import { mapSilhouette } from './map-silhouette.ts';

/** Width of the whole radar in plate units; world points map into it with `worldToRendered`. */
export const RADAR_PLATE_SIZE = 100;

/** What an SVG plate of a map draws: CS2's radar image when there is one, else the walkable silhouette. */
export type RadarPlateFrame = { viewBox: string; base: RadarRect; width: number; height: number; image: boolean; silhouette: string | null };

export function radarPlateFrame(geometry: TacticalGeometry, radar: { naturalWidth: number; naturalHeight: number } | null): RadarPlateFrame | null {
  if (radar) {
    const width = (RADAR_PLATE_SIZE * radar.naturalWidth) / geometry.calibration.size;
    const height = (RADAR_PLATE_SIZE * radar.naturalHeight) / geometry.calibration.size;
    return { viewBox: `0 0 ${width} ${height}`, base: { x: 0, y: 0, width, height }, width, height, image: true, silhouette: null };
  }
  const silhouette = mapSilhouette(geometry, RADAR_PLATE_SIZE);
  if (silhouette === null) return null;
  const [x = 0, y = 0, width = RADAR_PLATE_SIZE, height = RADAR_PLATE_SIZE] = silhouette.viewBox.split(' ').map(Number);
  return { viewBox: silhouette.viewBox, base: { x, y, width, height }, width, height, image: false, silhouette: silhouette.path };
}
