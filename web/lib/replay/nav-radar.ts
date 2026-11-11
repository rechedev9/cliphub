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
 * The radar's own colours, sampled from Skybox EDGE's plate: a near-black void,
 * one mid grey for the walkable floor that only moves a little with height,
 * a darker grey for obstacles inside it, and muted tints for sites and spawns.
 */
export const NAV_RADAR_PALETTE = {
  void: '#161916',
  low: [84, 84, 84],
  high: [122, 122, 122],
  obstacle: [44, 44, 44],
  /** Bombsites, the CT spawn's blue and the T spawn's brown. */
  site: '#5e3a37',
  spawn: '#2f394a',
  spawnT: '#5a4b3c',
} as const satisfies { void: string; low: Rgb8; high: Rgb8; obstacle: Rgb8; site: string; spawn: string; spawnT: string };

/** The height range one floor's tone spans, with stray areas trimmed off both ends. */
export function toneRange(polygons: readonly NavPolygon[]): { minZ: number; maxZ: number } {
  const heights = polygons.map((polygon) => polygon.z).sort((a, b) => a - b);
  return { minZ: quantile(heights, Z_TRIM), maxZ: quantile(heights, 1 - Z_TRIM) };
}

/** Walkable ground within this many world units of a bombsite or spawn centre takes its tint. */
const SITE_REACH = 300;
const SPAWN_REACH = 380;
/** Only the ground at the height of the place: the floor above or under it stays grey. */
const TINT_HEIGHT = 150;

type Callouts = TacticalGeometry['callouts'];

const regionCache = new WeakMap<Callouts, readonly PlaceRegion[]>();

/** A bombsite or spawn as one block: the box round its areas and its tint. */
export type PlaceRegion = { minX: number; minY: number; maxX: number; maxY: number; color: string; level: RadarLevel };

function placeKind(name: string): { color: string; reach: number } | null {
  const key = name.toLowerCase().replace(/[^a-z]/g, '');
  if (key === 'bombsitea' || key === 'bombsiteb') return { color: NAV_RADAR_PALETTE.site, reach: SITE_REACH };
  if (key === 'ctspawn') return { color: NAV_RADAR_PALETTE.spawn, reach: SPAWN_REACH };
  if (key === 'tspawn') return { color: NAV_RADAR_PALETTE.spawnT, reach: SPAWN_REACH };
  return null;
}

type PlaceArea = { color: string; level: RadarLevel; polygons: NavPolygon[] };

function placeAreas(model: NavRadarModel, callouts: Callouts, lowerAltitudeMax: number | undefined): PlaceArea[] {
  const found: PlaceArea[] = [];
  for (const callout of callouts) {
    const kind = placeKind(callout.name);
    if (kind === null) continue;
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
    const base = nearest.z;
    const polygons = onFloor.filter((polygon) => Math.hypot(polygon.cx - x, polygon.cy - y) <= kind.reach && Math.abs(polygon.z - base) <= TINT_HEIGHT);
    found.push({ color: kind.color, level: callout.level, polygons });
  }
  return found;
}

/** Each bombsite and spawn as the box round its areas, so the tint reads as one clean block like EDGE's. */
export function placeRegions(model: NavRadarModel, callouts: Callouts, lowerAltitudeMax: number | undefined): readonly PlaceRegion[] {
  const known = regionCache.get(callouts);
  if (known !== undefined) return known;
  const regions: PlaceRegion[] = [];
  for (const area of placeAreas(model, callouts, lowerAltitudeMax)) {
    const bounds = floorBounds(area.polygons);
    if (bounds !== null) regions.push({ ...bounds, color: area.color, level: area.level });
  }
  regionCache.set(callouts, regions);
  return regions;
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

/** Which floor a polygon belongs to on a split-level map, like `levelForAltitude`. */
export function polygonLevel(z: number, lowerAltitudeMax: number | undefined): 'default' | 'lower' {
  return lowerAltitudeMax !== undefined && z <= lowerAltitudeMax ? 'lower' : 'default';
}
