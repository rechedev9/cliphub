import { RADAR_LEVELS } from './api/tactical.ts';
import type {
  RadarBounds,
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

/** World-space breathing room around the play bounds, as a share of its longer axis. */
const RADAR_VIEW_MARGIN = 0.03;
/**
 * Floor for that margin as a share of the square. `radar-draw` sizes a blip's
 * view cone at 4.4 radii (0.046 of the square) and its name chip at 2.6 radii
 * plus a 1.5-line 0.016 glyph (0.051), so a player standing exactly on the
 * bounds keeps their cone, health ring and chip inside the window. A chip that
 * still would not fit is pulled in by `placeTag`, which clamps to this window
 * rather than to the square, so it is nudged over the outermost cells instead
 * of being sliced by the plate's border.
 */
const RADAR_VIEW_MARK_MARGIN = 0.05;
/** The plate is a window, never a sliver: the crop's aspect stays in this band. */
const RADAR_VIEW_MIN_ASPECT = 3 / 4;
const RADAR_VIEW_MAX_ASPECT = 16 / 9;
/** The whole square: the answer whenever a geometry cannot be framed. */
const RADAR_VIEW_FULL: RadarRect = { x: 0, y: 0, width: 1, height: 1 };

/**
 * Sample mass each edge of an axis may shed before the outermost remaining
 * cell fixes the play bounds. A stray sample — a spectator camera, a teleport
 * glitch, a body flung out of the world — is a handful of samples in an
 * otherwise empty cell, while any position that mattered was held for whole
 * seconds and weighs hundreds; 0.05% of a demo sits between the two, and the
 * absolute floor keeps the trim meaningful on very short demos.
 */
const OCCUPANCY_TRIM_SHARE = 0.0005;
const OCCUPANCY_TRIM_FLOOR = 2;

/** The trimmed [min, max] cell span of one axis' occupancy mass. */
function trimmedCellSpan(mass: Map<number, number>, trim: number): { min: number; max: number } {
  const entries = [...mass.entries()].sort((a, b) => a[0] - b[0]);
  let lo = 0;
  let hi = entries.length - 1;
  let shed = 0;
  while (lo < hi && shed + entries[lo][1] <= trim) {
    shed += entries[lo][1];
    lo += 1;
  }
  shed = 0;
  while (hi > lo && shed + entries[hi][1] <= trim) {
    shed += entries[hi][1];
    hi -= 1;
  }
  return { min: entries[lo][0], max: entries[hi][0] };
}

/**
 * The world-space play bounds re-derived from the occupancy grid instead of
 * `geometry.bounds`. The server's bounds are the unconditional min/max of every
 * sample, so one out-of-world sample stretches them to the whole square and the
 * view window degenerates; the grid carries a per-cell sample count, which lets
 * each axis shed that stray mass from its edges before the span is read. `null`
 * when the geometry has no weighted cells to read.
 */
export function occupancyBounds(geometry: TacticalGeometry): RadarBounds | null {
  if (!(geometry.cell_size > 0)) return null;
  const xMass = new Map<number, number>();
  const yMass = new Map<number, number>();
  let total = 0;
  for (const level of geometry.levels) {
    for (const [x, y, weight] of level.cells) {
      if (!(weight > 0)) continue;
      xMass.set(x, (xMass.get(x) ?? 0) + weight);
      yMass.set(y, (yMass.get(y) ?? 0) + weight);
      total += weight;
    }
  }
  if (total === 0) return null;
  const trim = Math.max(OCCUPANCY_TRIM_FLOOR, total * OCCUPANCY_TRIM_SHARE);
  const xs = trimmedCellSpan(xMass, trim);
  const ys = trimmedCellSpan(yMass, trim);
  // Cells are indexed by floor(world / cell_size), so the far edge of the
  // outermost surviving cell is one whole cell past its index.
  return {
    min_x: xs.min * geometry.cell_size,
    min_y: ys.min * geometry.cell_size,
    max_x: (xs.max + 1) * geometry.cell_size,
    max_y: (ys.max + 1) * geometry.cell_size,
  };
}

/** Grows one axis of the window around its centre, keeping it inside [0, 1]. */
function growAxis(
  origin: number,
  extent: number,
  target: number,
): { origin: number; extent: number } {
  if (!(target > extent)) return { origin, extent };
  const grown = Math.min(target, 1);
  const centre = origin + extent / 2;
  return { origin: Math.min(Math.max(centre - grown / 2, 0), 1 - grown), extent: grown };
}

/**
 * The window of the native radar square the demo actually happened in, as
 * fractions of that square (a `RadarRect` of a `renderedSize = 1` radar).
 *
 * This is a viewport, never a transform. The caller scales one square by
 * `1 / width` and pans it, so the baked map, the cells, the callouts, the
 * trails, the event marks and the blips all move together and a blip lands on
 * exactly the cell it lands on without a crop. A geometry that cannot be framed
 * answers the whole square, which reduces the caller's arithmetic to what it
 * does today.
 */
export function radarViewRect(geometry: TacticalGeometry): RadarRect {
  const { calibration } = geometry;
  if (!isCalibrationUsable(calibration)) return RADAR_VIEW_FULL;
  if (!(geometry.cell_size > 0)) return RADAR_VIEW_FULL;
  // The occupancy grid is outlier-trimmed; the raw server bounds are only the
  // fallback for a geometry that carries no weighted cells.
  const bounds = occupancyBounds(geometry) ?? geometry.bounds;
  // Mirrors `radarmap.Bounds.Empty`, and rejects NaN on the way through.
  if (!(bounds.max_x > bounds.min_x) || !(bounds.max_y > bounds.min_y)) return RADAR_VIEW_FULL;

  const rect = worldRectToRadarRect(
    calibration,
    { minX: bounds.min_x, minY: bounds.min_y, maxX: bounds.max_x, maxY: bounds.max_y },
    1,
  );
  // One cell of pad keeps the fallback bounds — raw sample min/max, whose grid
  // reaches a whole cell past the outermost sample — inside the window; the mark
  // floor covers the furniture a blip standing on that edge draws around itself.
  const longer = Math.max(bounds.max_x - bounds.min_x, bounds.max_y - bounds.min_y);
  const worldPad = geometry.cell_size + RADAR_VIEW_MARGIN * longer;
  const pad = Math.max(worldPad / (calibration.scale * calibration.size), RADAR_VIEW_MARK_MARGIN);

  const left = Math.min(Math.max(rect.x - pad, 0), 1);
  const right = Math.min(Math.max(rect.x + rect.width + pad, 0), 1);
  const top = Math.min(Math.max(rect.y - pad, 0), 1);
  const bottom = Math.min(Math.max(rect.y + rect.height + pad, 0), 1);
  let x = left;
  let y = top;
  let width = right - left;
  let height = bottom - top;
  if (!(width > 0) || !(height > 0)) return RADAR_VIEW_FULL;

  // The short axis grows rather than the long one shrinking, so the clamp never
  // crops something the margin just made room for.
  if (width / height > RADAR_VIEW_MAX_ASPECT) {
    const grown = growAxis(y, height, width / RADAR_VIEW_MAX_ASPECT);
    y = grown.origin;
    height = grown.extent;
  } else if (width / height < RADAR_VIEW_MIN_ASPECT) {
    const grown = growAxis(x, width, height * RADAR_VIEW_MIN_ASPECT);
    x = grown.origin;
    width = grown.extent;
  }
  return { x, y, width, height };
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
