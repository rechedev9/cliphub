import type { MapGeometry } from '../api/map-geometry.ts';
import type { RadarLevel, TacticalGeometry } from '../api/tactical.ts';
import type { NavPatch } from './nav-patches.ts';

/**
 * The stylized radar drawn from the map's nav mesh: one flat polygon per
 * walkable area, toned by height so higher floors read lighter.
 */

/**
 * One nav area: flat world x,y pairs, its mean height, the lowest and highest of its corners, and its centre.
 * A patch names the floor it belongs to in `pane`; an area leaves it to its height.
 */
export type NavPolygon = { points: Float64Array; z: number; lowZ: number; highZ: number; cx: number; cy: number; pane?: RadarLevel };

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

/**
 * World units every polygon is grown by when it is painted. The mesh stops a few units short of walls and
 * leaves hairline gaps between neighbours; one grown outline closes both, with round corners.
 */
export const NAV_GROW = 8;

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
  const heights = polygons.map((polygon) => polygon.z);
  const bounds = floorBounds(polygons);
  if (bounds === null) return null;
  return { polygons, minZ: quantile(heights, Z_TRIM), maxZ: quantile(heights, 1 - Z_TRIM), bounds };
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

/**
 * The radar's own colours, flat like Skybox EDGE's: a black plate and one mid
 * grey for every walkable floor of both levels. Height only moves the grey
 * within a narrow band, so steps still read without a second colour.
 */
export const NAV_RADAR_PALETTE = {
  void: '#0b0c0e',
  outline: '#050506',
  low: [90, 90, 90],
  high: [107, 107, 107],
  /** Bombsites, and the CT spawn's blue tint. */
  site: '#4a2323',
  spawn: '#1f2c44',
} as const satisfies { void: string; outline: string; low: Rgb8; high: Rgb8; site: string; spawn: string };

/** The height range one floor's tone spans, with stray areas trimmed off both ends. */
export function toneRange(polygons: readonly NavPolygon[]): { minZ: number; maxZ: number } {
  const heights = polygons.map((polygon) => polygon.z).sort((a, b) => a - b);
  return { minZ: quantile(heights, Z_TRIM), maxZ: quantile(heights, 1 - Z_TRIM) };
}

/** Walkable ground within this many world units of a bombsite or spawn centre takes its tint. */
const SITE_REACH = 400;
const SPAWN_REACH = 460;
/** Only the ground at the height of the place: the floor above or under it stays grey. */
const TINT_HEIGHT = 150;

type Callouts = TacticalGeometry['callouts'];

const tintCache = new WeakMap<Callouts, Map<NavPolygon, string>>();

/**
 * The nav areas that make a bombsite or the CT spawn, each with its tint, found
 * from the places the demo recorded: the areas on the place's floor, near its
 * centre and at the height of the nearest ground.
 */
export function placeTints(model: NavRadarModel, callouts: Callouts, lowerAltitudeMax: number | undefined): Map<NavPolygon, string> {
  const known = tintCache.get(callouts);
  if (known !== undefined) return known;
  const tints = new Map<NavPolygon, string>();
  for (const callout of callouts) {
    const name = callout.name.toLowerCase().replace(/[^a-z]/g, '');
    const isSite = name === 'bombsitea' || name === 'bombsiteb';
    if (!isSite && name !== 'ctspawn') continue;
    const reach = isSite ? SITE_REACH : SPAWN_REACH;
    const [x, y] = callout.center;
    const onFloor = model.polygons.filter((polygon) => (polygon.pane ?? polygonLevel(polygon.z, lowerAltitudeMax)) === callout.level);
    let nearest: NavPolygon | null = null;
    let nearestDistance = Number.POSITIVE_INFINITY;
    for (const polygon of onFloor) {
      const distance = Math.hypot(polygon.cx - x, polygon.cy - y);
      if (distance < nearestDistance) {
        nearest = polygon;
        nearestDistance = distance;
      }
    }
    if (nearest === null) continue;
    for (const polygon of onFloor) {
      if (Math.hypot(polygon.cx - x, polygon.cy - y) <= reach && Math.abs(polygon.z - nearest.z) <= TINT_HEIGHT) tints.set(polygon, isSite ? NAV_RADAR_PALETTE.site : NAV_RADAR_PALETTE.spawn);
    }
  }
  tintCache.set(callouts, tints);
  return tints;
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

function segmentDistance(options: { x: number; y: number; ax: number; ay: number; bx: number; by: number }): number {
  const { x, y, ax, ay, bx, by } = options;
  const dx = bx - ax;
  const dy = by - ay;
  const length = dx * dx + dy * dy;
  const t = length > 0 ? Math.min(1, Math.max(0, ((x - ax) * dx + (y - ay) * dy) / length)) : 0;
  return Math.hypot(x - ax - t * dx, y - ay - t * dy);
}

/**
 * Whether the plate paints ground at a world point: inside one of `polygons`, or
 * within `NAV_GROW` of its edge, which is how far `paintFloors` grows each outline.
 */
export function isPainted(options: { polygons: readonly NavPolygon[]; x: number; y: number }): boolean {
  const { polygons, x, y } = options;
  return polygons.some(({ points }) => {
    let inside = false;
    for (let i = 0, j = points.length - 2; i < points.length; j = i, i += 2) {
      if (segmentDistance({ x, y, ax: points[j], ay: points[j + 1], bx: points[i], by: points[i + 1] }) <= NAV_GROW) return true;
      if (points[i + 1] > y !== points[j + 1] > y && x < ((points[j] - points[i]) * (y - points[i + 1])) / (points[j + 1] - points[i + 1]) + points[i]) inside = !inside;
    }
    return inside;
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

/** Which floor a polygon belongs to on a split-level map, like `levelForAltitude`. */
export function polygonLevel(z: number, lowerAltitudeMax: number | undefined): 'default' | 'lower' {
  return lowerAltitudeMax !== undefined && z <= lowerAltitudeMax ? 'lower' : 'default';
}
