'use client';

import { useEffect, useState } from 'react';
import { radarImageUrl, type RadarLayer } from '@/lib/api/game-assets';

// One decode per radar for the session; a failed load (CS2 missing or not set up yet) is forgotten, so the next view retries.
const cache = new Map<string, Promise<HTMLImageElement | null>>();

export function loadRadar(map: string, layer: RadarLayer): Promise<HTMLImageElement | null> {
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
  void pending.then((image) => {
    if (image === null && cache.get(url) === pending) cache.delete(url);
  });
  return pending;
}

/**
 * A map's radar as a decoded image for a canvas: undefined while it loads, null
 * when Studio ships none for the map and CS2 is not installed or has none either.
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
