import { getMapGeometry } from '@/lib/api/map-geometry';
import { navRadarModel } from '@/lib/replay/nav-radar';
import { NAV_PATCHES } from '@/lib/replay/nav-patches';
import type { NavRadarModel } from '@/lib/replay/nav-radar';
import { loadRadar } from '@/hooks/use-radar-image';
import type { MapArt } from './radar-bake';

/**
 * What a 2D view of a map draws, loaded once per map for the session: the radar
 * image it paints and the nav mesh that frames each floor.
 * A failed load answers null and is forgotten, so the next round tries again.
 */

const navModels = new Map<string, Promise<NavRadarModel | null>>();

function loadNavRadar(map: string): Promise<NavRadarModel | null> {
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

/** The plate of a map: the radar of each floor (the play-derived grid without one) and the nav mesh for the framing. */
export async function loadMapPlate(map: string, split: boolean): Promise<{ model: NavRadarModel | null; art: MapArt }> {
  const [model, upper, lower] = await Promise.all([loadNavRadar(map), loadRadar(map, 'upper'), split ? loadRadar(map, 'lower') : null]);
  return { model, art: upper === null ? { kind: 'grid' } : { kind: 'radar', images: { default: upper, lower } } };
}

/**
 * Starts the loads of `loadMapPlate` that need only the map name, before its first round arrives. The lower
 * floor waits for the round: only its calibration says whether the map has one.
 */
export function preloadMapPlate(map: string): void {
  void loadNavRadar(map);
  void loadRadar(map, 'upper');
}
