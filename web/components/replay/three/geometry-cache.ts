import { getMapGeometry } from '@/lib/api/map-geometry';
import type { MapGeometry } from '@/lib/api/map-geometry';
import { getMapScene } from '@/lib/api/map-scene';
import type { MapScene } from '@/lib/api/map-scene';

/** One read per map for the session; a failed read is forgotten so "Reintentar" reads again. */
const cache = new Map<string, Promise<MapGeometry>>();
const navCache = new Map<string, Promise<MapGeometry | null>>();

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

/** The walkable floor the 3D view cuts roofs over; null when it cannot be read, so the view falls back to a flat cut. */
export function loadNavGeometry(map: string): Promise<MapGeometry | null> {
  const cached = navCache.get(map);
  if (cached !== undefined) return cached;
  const pending = getMapGeometry(map, { source: 'nav' }).catch(() => null);
  navCache.set(map, pending);
  void pending.then((nav) => {
    if (nav === null && navCache.get(map) === pending) navCache.delete(map);
  });
  return pending;
}

/** The last map's render scene only: one weighs up to 200 MB, so switching maps drops the previous one. */
let sceneCache: { map: string; pending: Promise<MapScene>; share: number; listeners: Set<(share: number) => void> } | null = null;

/** Every caller hears the read's progress, including one that joins a read already under way. */
export function loadMapScene(map: string, onProgress?: (share: number) => void): Promise<MapScene> {
  if (sceneCache?.map === map) {
    if (onProgress !== undefined) {
      sceneCache.listeners.add(onProgress);
      onProgress(sceneCache.share);
    }
    return sceneCache.pending;
  }
  const listeners = new Set<(share: number) => void>(onProgress === undefined ? [] : [onProgress]);
  const entry = {
    map,
    share: 0,
    listeners,
    pending: getMapScene(map, (share) => {
      entry.share = share;
      for (const listener of listeners) listener(share);
    }),
  };
  sceneCache = entry;
  entry.pending.then(
    () => listeners.clear(),
    () => {
      listeners.clear();
      if (sceneCache === entry) sceneCache = null;
    },
  );
  return entry.pending;
}
