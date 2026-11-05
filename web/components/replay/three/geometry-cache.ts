import { getMapGeometry } from '@/lib/api/map-geometry';
import type { MapGeometry } from '@/lib/api/map-geometry';

/** One read per map for the session; a failed read is forgotten so "Reintentar" reads again. */
const cache = new Map<string, Promise<MapGeometry>>();

export function loadMapGeometry(map: string): Promise<MapGeometry> {
  const cached = cache.get(map);
  if (cached !== undefined) return cached;
  const pending = getMapGeometry(map);
  cache.set(map, pending);
  pending.catch(() => {
    if (cache.get(map) === pending) cache.delete(map);
  });
  return pending;
}
