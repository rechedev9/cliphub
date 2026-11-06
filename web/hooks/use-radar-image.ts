'use client';

import { useEffect, useState } from 'react';
import { radarImageUrl, type RadarLayer } from '@/lib/api/game-assets';

// One decode per radar for the session; a failed load (CS2 not installed) is remembered as null.
const cache = new Map<string, Promise<HTMLImageElement | null>>();

function loadRadar(map: string, layer: RadarLayer): Promise<HTMLImageElement | null> {
  const url = radarImageUrl(map, layer);
  const known = cache.get(url);
  if (known !== undefined) return known;
  const pending = new Promise<HTMLImageElement | null>((resolve) => {
    const image = new Image();
    image.decoding = 'async';
    image.onload = () => resolve(image);
    image.onerror = () => resolve(null);
    image.src = url;
  });
  cache.set(url, pending);
  return pending;
}

/**
 * CS2's own radar of a map as a decoded image for a canvas: undefined while it
 * loads, null when the game is not installed or has no radar for the map.
 */
export function useRadarImage(map: string, layer: RadarLayer = 'upper'): HTMLImageElement | null | undefined {
  const [image, setImage] = useState<{ key: string; value: HTMLImageElement | null } | null>(null);
  const key = `${map}|${layer}`;
  useEffect(() => {
    let active = true;
    if (map === '') return undefined;
    void loadRadar(map, layer).then((value) => {
      if (active) setImage({ key, value });
    });
    return () => {
      active = false;
    };
  }, [map, layer, key]);
  if (map === '') return null;
  return image?.key === key ? image.value : undefined;
}
