import { radarImageUrl } from '@/lib/api/game-assets';
import type { RadarLayer } from '@/lib/api/game-assets';
import { getMapGeometry } from '@/lib/api/map-geometry';
import { navRadarModel } from '@/lib/replay/nav-radar';
import type { NavRadarModel } from '@/lib/replay/nav-radar';

/**
 * The 2D map's art, loaded once per map for the session: the stylized radar
 * from the nav mesh, and the game's own radar image when Studio serves it.
 * A failed load answers null and is forgotten, so the next round tries again.
 */

const navModels = new Map<string, Promise<NavRadarModel | null>>();
const radarImages = new Map<string, Promise<HTMLImageElement | null>>();

export function loadNavRadar(map: string): Promise<NavRadarModel | null> {
  const cached = navModels.get(map);
  if (cached !== undefined) return cached;
  const pending = getMapGeometry(map, { source: 'nav' })
    .then((geometry) => navRadarModel(geometry))
    .catch(() => null);
  navModels.set(map, pending);
  void pending.then((model) => {
    if (model === null && navModels.get(map) === pending) navModels.delete(map);
  });
  return pending;
}

/** The official overview of one layer; a missing layer or a missing CS2 answers null. */
export function loadRadarImage(map: string, layer: RadarLayer): Promise<HTMLImageElement | null> {
  const key = `${map}|${layer}`;
  const cached = radarImages.get(key);
  if (cached !== undefined) return cached;
  const pending = new Promise<HTMLImageElement | null>((resolve) => {
    const image = new Image();
    image.decoding = 'async';
    image.onload = () => resolve(image.naturalWidth > 0 ? image : null);
    image.onerror = () => resolve(null);
    image.src = radarImageUrl(map, layer);
  });
  radarImages.set(key, pending);
  return pending;
}
