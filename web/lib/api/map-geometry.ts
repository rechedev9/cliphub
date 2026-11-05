/**
 * Map geometry for the 3D replay (`internal/mapgeo`), read at runtime from the user's own CS2
 * install: `physics` is the decimated collision (walls, floors, props that stop players and
 * bullets; undersides dropped, winding not uniform, so draw it double-sided), `nav` the walkable
 * floor. World units, Z up, the same space as the tactical document's positions.
 */

/** Which geometry to read; `physics` falls back to `nav` when the collision cannot be read. */
export type MapGeometrySource = 'physics' | 'nav';

/** A world point `[x, y, z]`. */
export type MapVec3 = [number, number, number];

/** One nav area; `polygon` indexes vertices, counter-clockwise seen from +Z. Empty for physics. */
export type MapGeometryArea = { id: number; polygon: number[] };

/**
 * `vertices` is flat x,y,z and `triangles` flat vertex indices, ready for
 * `BufferGeometry.setAttribute('position', new BufferAttribute(vertices, 3))` and `setIndex`.
 */
export type MapGeometry = {
  map: string;
  source: MapGeometrySource;
  build: string;
  bounds: { min: MapVec3; max: MapVec3 };
  vertices: Float32Array;
  triangles: Uint32Array;
  areas: MapGeometryArea[];
  /** Why a physics request was answered with the nav mesh. */
  fallback?: string;
};

/** Same rule as the orchestrator's `geometryMapPattern`. */
export const MAP_GEOMETRY_NAME_RE = /^(de|cs|ar)_[a-z0-9_]{1,32}$/;

/**
 * `code` is `cs2_not_found`, `map_not_found` or `map_has_no_nav` (404), `map_geometry_unreadable`
 * (422), `invalid_request` (400) or the proxy's `service_unavailable` (503).
 */
export class MapGeometryError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'MapGeometryError';
    this.status = status;
    this.code = code;
  }
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function isNumberArray(value: unknown): value is number[] {
  return Array.isArray(value) && value.every((n) => typeof n === 'number' && Number.isFinite(n));
}

function isIndexArray(value: unknown, vertexCount: number): value is number[] {
  return Array.isArray(value) && value.every((n) => Number.isInteger(n) && n >= 0 && n < vertexCount);
}

function vec3(value: unknown): MapVec3 | null {
  if (!isNumberArray(value) || value.length !== 3) return null;
  return [value[0], value[1], value[2]];
}

function parseAreas(raw: unknown, vertexCount: number): MapGeometryArea[] | null {
  if (!Array.isArray(raw)) return null;
  const areas: MapGeometryArea[] = [];
  for (const row of raw) {
    if (!isRecord(row) || typeof row.id !== 'number' || !isIndexArray(row.polygon, vertexCount)) return null;
    areas.push({ id: row.id, polygon: row.polygon });
  }
  return areas;
}

/** The orchestrator's mesh body, or null when any part is malformed or out of range. */
export function parseMapGeometry(raw: unknown): MapGeometry | null {
  if (!isRecord(raw) || typeof raw.map !== 'string') return null;
  const source = raw.source === 'physics' || raw.source === 'nav' ? raw.source : null;
  if (source === null) return null;
  const bounds = isRecord(raw.bounds) ? { min: vec3(raw.bounds.min), max: vec3(raw.bounds.max) } : null;
  if (bounds === null || bounds.min === null || bounds.max === null) return null;
  if (!isNumberArray(raw.vertices) || raw.vertices.length % 3 !== 0) return null;
  const vertexCount = raw.vertices.length / 3;
  if (!isIndexArray(raw.triangles, vertexCount) || raw.triangles.length % 3 !== 0) return null;
  const areas = parseAreas(raw.areas, vertexCount);
  if (areas === null) return null;
  return {
    map: raw.map,
    source,
    build: typeof raw.build === 'string' ? raw.build : '',
    bounds: { min: bounds.min, max: bounds.max },
    vertices: Float32Array.from(raw.vertices),
    triangles: Uint32Array.from(raw.triangles),
    areas,
    ...(typeof raw.fallback === 'string' && raw.fallback !== '' ? { fallback: raw.fallback } : {}),
  };
}

export function isMapGeometrySource(value: string): value is MapGeometrySource {
  return value === 'physics' || value === 'nav';
}

/** `GET /api/maps/{map}/geometry?source=`; throws MapGeometryError with the orchestrator's code. */
export async function getMapGeometry(
  map: string,
  options: { source?: MapGeometrySource; signal?: AbortSignal } = {},
): Promise<MapGeometry> {
  if (!MAP_GEOMETRY_NAME_RE.test(map)) throw new MapGeometryError(`invalid map name: ${map}`, 400, 'invalid_request');
  const query = options.source === undefined ? '' : `?source=${options.source}`;
  const res = await fetch(`/api/maps/${encodeURIComponent(map)}/geometry${query}`, { signal: options.signal });
  const body: unknown = await res.json().catch(() => null);
  if (!res.ok) {
    const message = isRecord(body) && typeof body.error === 'string' ? body.error : `map geometry failed (${res.status})`;
    const code = isRecord(body) && typeof body.code === 'string' ? body.code : undefined;
    throw new MapGeometryError(message, res.status, code);
  }
  const geometry = parseMapGeometry(body);
  if (geometry === null) throw new MapGeometryError('malformed map geometry', 502);
  return geometry;
}
