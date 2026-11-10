import { getMapGeometry } from '@/lib/api/map-geometry';
import { navRadarModel } from '@/lib/replay/nav-radar';
import { NAV_PATCHES } from '@/lib/replay/nav-patches';
import type { NavRadarModel } from '@/lib/replay/nav-radar';

/**
 * The 2D map's art, loaded once per map for the session: the stylized radar
 * from the nav mesh.
 * A failed load answers null and is forgotten, so the next round tries again.
 */

const navModels = new Map<string, Promise<NavRadarModel | null>>();

export function loadNavRadar(map: string): Promise<NavRadarModel | null> {
  const cached = navModels.get(map);
  if (cached !== undefined) return cached;
  const pending = getMapGeometry(map, { source: 'nav' })
    .then((geometry) => navRadarModel(geometry, NAV_PATCHES[map]))
    .catch(() => null);
  navModels.set(map, pending);
  void pending.then((model) => {
    if (model === null && navModels.get(map) === pending) navModels.delete(map);
  });
  return pending;
}
