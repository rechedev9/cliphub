import { RADAR_LEVELS } from '../api/tactical.ts';
import type { RadarCalibration, RadarLevel } from '../api/tactical.ts';
import { radarProjection } from '../tactical-spatial.ts';
import type { RadarProjection } from '../tactical-spatial.ts';
import { levelForAltitude } from '../tactical-transform.ts';
import type { RadarRect } from '../tactical-transform.ts';

/**
 * Both floors of a two-floor map (Nuke, Vertigo) side by side on the 2D map:
 * where each floor's pane sits in the canvas and how the shared view projects
 * into it, so zoom and pan move both floors together.
 */

/** One floor's part of the map canvas, in its CSS pixels. */
export type FloorPane = { level: RadarLevel; x: number; y: number; width: number; height: number };

/** The line between the two panes, in CSS pixels. */
export const PANE_GAP = 2;

/** Upper floor first, as the game's own radar lists them. */
export const BOTH_FLOORS: readonly RadarLevel[] = [RADAR_LEVELS.default, RADAR_LEVELS.lower];

/**
 * The panes for `levels`: one floor takes the whole box; two sit side by side
 * when the box is wider than tall, else one over the other, upper first.
 */
export function floorPanes(box: { width: number; height: number }, levels: readonly RadarLevel[]): FloorPane[] {
  const width = Math.max(1, box.width);
  const height = Math.max(1, box.height);
  if (levels.length < 2) return [{ level: levels[0] ?? RADAR_LEVELS.default, x: 0, y: 0, width, height }];
  const [first, second] = levels;
  if (width >= height) {
    const half = Math.max(1, Math.floor((width - PANE_GAP) / 2));
    return [
      { level: first, x: 0, y: 0, width: half, height },
      { level: second, x: width - half, y: 0, width: half, height },
    ];
  }
  const half = Math.max(1, Math.floor((height - PANE_GAP) / 2));
  return [
    { level: first, x: 0, y: 0, width, height: half },
    { level: second, x: 0, y: height - half, width, height: half },
  ];
}

/** The shared view fitted into one pane, in the canvas's coordinates. */
export function paneProjection(pane: FloorPane, view: RadarRect): RadarProjection {
  const local = radarProjection(pane, view);
  return { size: local.size, offsetX: local.offsetX + pane.x, offsetY: local.offsetY + pane.y };
}

/** The same projection relative to the pane's own corner, for a layer laid over just that pane. */
export function localProjection(pane: FloorPane, projection: RadarProjection): RadarProjection {
  return { size: projection.size, offsetX: projection.offsetX - pane.x, offsetY: projection.offsetY - pane.y };
}

/** The pane under a canvas point, with the point relative to it; null on the line between them. */
export function paneAt(panes: readonly FloorPane[], x: number, y: number): { pane: FloorPane; x: number; y: number } | null {
  const pane = panes.find((entry) => x >= entry.x && x < entry.x + entry.width && y >= entry.y && y < entry.y + entry.height);
  return pane === undefined ? null : { pane, x: x - pane.x, y: y - pane.y };
}

/** What stands on one floor: everything else belongs to the other pane. */
export function onFloorOf<T extends { z: number }>(calibration: RadarCalibration, level: RadarLevel, items: readonly T[]): T[] {
  return items.filter((item) => levelForAltitude(calibration, item.z) === level);
}
