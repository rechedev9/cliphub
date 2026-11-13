import type { MapGeometry } from '../api/map-geometry.ts';
import type { RadarLevel } from '../api/tactical.ts';
import type { NavPatch } from './nav-patches.ts';

/**
 * The map's walkable ground from its nav mesh: one flat polygon per area. The
 * 2D views frame each floor with it; the radar image is what they paint.
 */

/**
 * One nav area: flat world x,y pairs, its mean height, the lowest and highest of its corners, and its centre.
 * A patch names the floor it belongs to in `pane`; an area leaves it to its height.
 */
export type NavPolygon = { points: Float64Array; z: number; lowZ: number; highZ: number; cx: number; cy: number; pane?: RadarLevel };

export type NavRadarModel = {
  /** Lowest first. */
  polygons: NavPolygon[];
  bounds: { minX: number; minY: number; maxX: number; maxY: number };
};

function polygonOf(vertices: Float32Array, indices: readonly number[]): NavPolygon | null {
  if (indices.length < 3) return null;
  const points = new Float64Array(indices.length * 2);
  let z = 0;
  let lowZ = Number.POSITIVE_INFINITY;
  let highZ = Number.NEGATIVE_INFINITY;
  let cx = 0;
  let cy = 0;
  for (let i = 0; i < indices.length; i += 1) {
    const at = indices[i] * 3;
    points[i * 2] = vertices[at];
    points[i * 2 + 1] = vertices[at + 1];
    cx += vertices[at];
    cy += vertices[at + 1];
    z += vertices[at + 2];
    lowZ = Math.min(lowZ, vertices[at + 2]);
    highZ = Math.max(highZ, vertices[at + 2]);
  }
  return { points, z: z / indices.length, lowZ, highZ, cx: cx / indices.length, cy: cy / indices.length };
}

function patchPolygon(patch: NavPatch): NavPolygon | null {
  const count = patch.points.length / 2;
  const vertices = new Float32Array(count * 3);
  for (let i = 0; i < count; i += 1) {
    vertices[i * 3] = patch.points[i * 2];
    vertices[i * 3 + 1] = patch.points[i * 2 + 1];
    vertices[i * 3 + 2] = patch.z;
  }
  const polygon = polygonOf(vertices, Array.from({ length: count }, (_, i) => i));
  return polygon === null ? null : { ...polygon, pane: patch.level };
}

/** The radar model of a nav mesh with the map's patches, or null when it has nothing to draw. */
export function navRadarModel(geometry: Pick<MapGeometry, 'vertices' | 'triangles' | 'areas'>, patches: readonly NavPatch[] = []): NavRadarModel | null {
  const polygons: NavPolygon[] = [];
  if (geometry.areas.length > 0) {
    for (const area of geometry.areas) {
      const polygon = polygonOf(geometry.vertices, area.polygon);
      if (polygon !== null) polygons.push(polygon);
    }
  } else {
    for (let i = 0; i + 2 < geometry.triangles.length; i += 3) {
      const polygon = polygonOf(geometry.vertices, [geometry.triangles[i], geometry.triangles[i + 1], geometry.triangles[i + 2]]);
      if (polygon !== null) polygons.push(polygon);
    }
  }
  for (const patch of patches) {
    const polygon = patchPolygon(patch);
    if (polygon !== null) polygons.push(polygon);
  }
  if (polygons.length === 0) return null;
  polygons.sort((a, b) => a.z - b.z);
  const bounds = floorBounds(polygons);
  if (bounds === null) return null;
  return { polygons, bounds };
}

/**
 * The nav areas on one floor of a two-floor map; a one-floor map answers all of
 * them. A long ramp area whose corners lie on both sides of the cut belongs to
 * both floors, so a player on its low part has ground under them on the lower
 * pane and one on its high part on the upper pane.
 */
export function floorPolygons(model: NavRadarModel, lowerAltitudeMax: number | undefined, level: RadarLevel): NavPolygon[] {
  if (lowerAltitudeMax === undefined) return model.polygons;
  return model.polygons.filter((polygon) => {
    if (polygon.pane !== undefined) return polygon.pane === level;
    return level === 'lower' ? polygon.lowZ <= lowerAltitudeMax : polygon.highZ > lowerAltitudeMax;
  });
}

/** The world rectangle one floor's walkable areas cover, or null when it has none. */
export function floorBounds(polygons: readonly NavPolygon[]): NavRadarModel['bounds'] | null {
  if (polygons.length === 0) return null;
  let minX = Number.POSITIVE_INFINITY;
  let minY = Number.POSITIVE_INFINITY;
  let maxX = Number.NEGATIVE_INFINITY;
  let maxY = Number.NEGATIVE_INFINITY;
  for (const { points } of polygons) {
    for (let i = 0; i < points.length; i += 2) {
      minX = Math.min(minX, points[i]);
      maxX = Math.max(maxX, points[i]);
      minY = Math.min(minY, points[i + 1]);
      maxY = Math.max(maxY, points[i + 1]);
    }
  }
  return { minX, minY, maxX, maxY };
}
