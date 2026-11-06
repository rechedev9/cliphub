import type { MapGeometry } from '../api/map-geometry.ts';

/**
 * The stylized radar drawn from the map's nav mesh: one flat polygon per
 * walkable area, toned by height so higher floors read lighter.
 */

/** One nav area: flat world x,y pairs and its mean height. */
export type NavPolygon = { points: Float64Array; z: number };

export type NavRadarModel = {
  /** Lowest floor first, so higher floors paint over the ones under them. */
  polygons: NavPolygon[];
  /** Height range the tone spans, with stray areas trimmed off both ends. */
  minZ: number;
  maxZ: number;
  bounds: { minX: number; minY: number; maxX: number; maxY: number };
};

/** Share of areas ignored at each end of the height range, so a lone catwalk does not flatten the rest. */
const Z_TRIM = 0.02;

function quantile(sorted: readonly number[], q: number): number {
  if (sorted.length === 0) return 0;
  const at = Math.min(sorted.length - 1, Math.max(0, Math.round(q * (sorted.length - 1))));
  return sorted[at];
}

function polygonOf(vertices: Float32Array, indices: readonly number[]): NavPolygon | null {
  if (indices.length < 3) return null;
  const points = new Float64Array(indices.length * 2);
  let z = 0;
  for (let i = 0; i < indices.length; i += 1) {
    const at = indices[i] * 3;
    points[i * 2] = vertices[at];
    points[i * 2 + 1] = vertices[at + 1];
    z += vertices[at + 2];
  }
  return { points, z: z / indices.length };
}

/** The radar model of a nav mesh, or null when it has nothing to draw. */
export function navRadarModel(geometry: Pick<MapGeometry, 'vertices' | 'triangles' | 'areas'>): NavRadarModel | null {
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
  if (polygons.length === 0) return null;
  polygons.sort((a, b) => a.z - b.z);
  const heights = polygons.map((polygon) => polygon.z);
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
  return { polygons, minZ: quantile(heights, Z_TRIM), maxZ: quantile(heights, 1 - Z_TRIM), bounds: { minX, minY, maxX, maxY } };
}

/** 0 at the lowest floor, 1 at the highest, eased so mid floors keep contrast. */
export function heightTone(z: number, minZ: number, maxZ: number): number {
  const span = maxZ - minZ;
  if (!(span > 0)) return 0.5;
  const t = Math.min(1, Math.max(0, (z - minZ) / span));
  return Math.sqrt(t);
}

export type Rgb8 = readonly [number, number, number];

/** The floor colour at `tone` between the low and the high grey. */
export function toneColor(tone: number, low: Rgb8, high: Rgb8): string {
  const t = Math.min(1, Math.max(0, tone));
  const channel = (i: number): number => Math.round(low[i] + (high[i] - low[i]) * t);
  return `rgb(${channel(0)}, ${channel(1)}, ${channel(2)})`;
}

/** The radar's own colours: a dark plate in both themes, like the game's radar image. */
export const NAV_RADAR_PALETTE = {
  void: '#0b0c0e',
  outline: '#050506',
  low: [52, 57, 63],
  high: [156, 162, 170],
} as const satisfies { void: string; outline: string; low: Rgb8; high: Rgb8 };

/** Which floor a polygon belongs to on a split-level map, like `levelForAltitude`. */
export function polygonLevel(z: number, lowerAltitudeMax: number | undefined): 'default' | 'lower' {
  return lowerAltitudeMax !== undefined && z <= lowerAltitudeMax ? 'lower' : 'default';
}
