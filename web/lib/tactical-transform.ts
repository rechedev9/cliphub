import { RADAR_LEVELS } from './api/tactical.ts';
import type {
  RadarCalibration,
  RadarLevel,
  TacticalGeometry,
  TacticalGeometryCell,
  TacticalGeometryLevel,
} from './api/tactical.ts';

/**
 * Pure world -> radar geometry for the tactical view, mirroring
 * `internal/radarmap` and `MapGeometry`'s occupancy grid.
 *
 * The transform is CS2's own: `(world - pos)/scale` yields native radar pixels,
 * with the Y axis inverted because world Y grows north while radar Y grows down.
 * Everything here is a function of its arguments: no fetching, no caching, no
 * DOM, so a renderer can call it per frame.
 */

/** A point in native or rendered radar pixels; y grows downward. */
export type RadarPoint = { x: number; y: number };

/** A point in CS2 world units; y grows north. */
export type WorldPoint = { x: number; y: number };

/** An axis-aligned rectangle in world units. */
export type WorldRect = { minX: number; minY: number; maxX: number; maxY: number };

/** An axis-aligned rectangle in radar pixels, ready for canvas or CSS. */
export type RadarRect = { x: number; y: number; width: number; height: number };

/** Reports whether a calibration can drive a transform (`radarmap.Calibration.Valid`). */
export function isCalibrationUsable(calibration: RadarCalibration): boolean {
  return calibration.scale > 0 && calibration.size > 0;
}

function assertUsable(calibration: RadarCalibration): void {
  if (!isCalibrationUsable(calibration)) {
    throw new Error(
      `tactical transform: calibration for "${calibration.map}" has scale ${calibration.scale} and size ${calibration.size}`,
    );
  }
}

function assertRenderedSize(renderedSize: number): void {
  if (!(renderedSize > 0) || !Number.isFinite(renderedSize)) {
    throw new Error(`tactical transform: rendered size ${renderedSize} must be positive`);
  }
}

/**
 * Converts world coordinates to native radar pixels. The Y axis inverts: world Y
 * grows north, radar Y grows down.
 */
export function worldToPixel(
  calibration: RadarCalibration,
  worldX: number,
  worldY: number,
): RadarPoint {
  assertUsable(calibration);
  return {
    x: (worldX - calibration.pos_x) / calibration.scale,
    y: (calibration.pos_y - worldY) / calibration.scale,
  };
}

/** The inverse of `worldToPixel`. */
export function pixelToWorld(
  calibration: RadarCalibration,
  pixelX: number,
  pixelY: number,
): WorldPoint {
  assertUsable(calibration);
  return {
    x: pixelX * calibration.scale + calibration.pos_x,
    y: calibration.pos_y - pixelY * calibration.scale,
  };
}

/**
 * Rendered pixels per native radar pixel. A 1024-pixel overview drawn 512 wide
 * scales by 0.5, and every length — dot radius, line width, cell size — must be
 * multiplied by it to stay proportional.
 */
export function radarScaleFactor(calibration: RadarCalibration, renderedSize: number): number {
  assertUsable(calibration);
  assertRenderedSize(renderedSize);
  return renderedSize / calibration.size;
}

/** Converts world coordinates straight to pixels of a `renderedSize` square radar. */
export function worldToRendered(
  calibration: RadarCalibration,
  worldX: number,
  worldY: number,
  renderedSize: number,
): RadarPoint {
  const factor = radarScaleFactor(calibration, renderedSize);
  const pixel = worldToPixel(calibration, worldX, worldY);
  return { x: pixel.x * factor, y: pixel.y * factor };
}

/** Converts rendered radar pixels back to world coordinates. */
export function renderedToWorld(
  calibration: RadarCalibration,
  renderedX: number,
  renderedY: number,
  renderedSize: number,
): WorldPoint {
  const factor = radarScaleFactor(calibration, renderedSize);
  return pixelToWorld(calibration, renderedX / factor, renderedY / factor);
}

/**
 * Reports which vertical section an altitude belongs to. Maps without a lower
 * section always answer `default`, mirroring `radarmap.Calibration.Level`.
 */
export function levelForAltitude(calibration: RadarCalibration, worldZ: number): RadarLevel {
  const lower = calibration.lower_altitude_max;
  return lower !== undefined && worldZ <= lower ? RADAR_LEVELS.lower : RADAR_LEVELS.default;
}

/** Reports whether the map is drawn as two overview images. */
export function isMultiLevelMap(calibration: RadarCalibration): boolean {
  return calibration.lower_altitude_max !== undefined;
}

/** Returns one vertical section's occupancy grid, or undefined when it has none. */
export function geometryLevel(
  geometry: TacticalGeometry,
  level: RadarLevel,
): TacticalGeometryLevel | undefined {
  return geometry.levels.find((candidate) => candidate.name === level);
}

function assertCellSize(geometry: TacticalGeometry): void {
  if (!(geometry.cell_size > 0)) {
    throw new Error(`tactical transform: geometry cell size ${geometry.cell_size} must be positive`);
  }
}

/**
 * The world rectangle an occupancy cell covers. Cells are indexed by
 * `floor(world / cell_size)`, so cell N spans `[N * size, (N+1) * size)`.
 */
export function cellWorldRect(
  geometry: TacticalGeometry,
  cellX: number,
  cellY: number,
): WorldRect {
  assertCellSize(geometry);
  const size = geometry.cell_size;
  return {
    minX: cellX * size,
    minY: cellY * size,
    maxX: (cellX + 1) * size,
    maxY: (cellY + 1) * size,
  };
}

/** The world centre of a cell, mirroring `MapGeometry.CellCenter`. */
export function cellWorldCenter(
  geometry: TacticalGeometry,
  cellX: number,
  cellY: number,
): WorldPoint {
  assertCellSize(geometry);
  return { x: (cellX + 0.5) * geometry.cell_size, y: (cellY + 0.5) * geometry.cell_size };
}

/**
 * Converts a world rectangle to a radar rectangle of a `renderedSize` square.
 * The Y inversion swaps the edges: the world's maximum Y is the rectangle's top.
 */
export function worldRectToRadarRect(
  calibration: RadarCalibration,
  rect: WorldRect,
  renderedSize: number,
): RadarRect {
  const topLeft = worldToRendered(calibration, rect.minX, rect.maxY, renderedSize);
  const bottomRight = worldToRendered(calibration, rect.maxX, rect.minY, renderedSize);
  return {
    x: topLeft.x,
    y: topLeft.y,
    width: bottomRight.x - topLeft.x,
    height: bottomRight.y - topLeft.y,
  };
}

/**
 * The radar rectangle one packed occupancy cell `[cellX, cellY, weight]` should
 * be drawn in. This is the whole map-drawing primitive: a level's cells, each
 * turned into a rect, is the walkable space the demo proved.
 */
export function cellRadarRect(
  geometry: TacticalGeometry,
  cell: TacticalGeometryCell,
  renderedSize: number,
): RadarRect {
  return worldRectToRadarRect(
    geometry.calibration,
    cellWorldRect(geometry, cell[0], cell[1]),
    renderedSize,
  );
}

/** The heaviest cell weight in a level, the denominator for an occupancy ramp. */
export function maxCellWeight(level: TacticalGeometryLevel): number {
  let max = 0;
  for (const cell of level.cells) {
    if (cell[2] > max) max = cell[2];
  }
  return max;
}

/**
 * Converts a CS2 yaw (degrees counter-clockwise from +X) into degrees clockwise
 * from the rendered +X axis, which is what a canvas rotation or a CSS
 * `rotate()` expects once the radar's Y axis has been inverted.
 */
export function yawToRadarAngle(yawDegrees: number): number {
  if (!Number.isFinite(yawDegrees)) return 0;
  return ((-yawDegrees % 360) + 360) % 360;
}
