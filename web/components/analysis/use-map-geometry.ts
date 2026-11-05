'use client';

import { useEffect, useState } from 'react';
import { getSpatialOverlay } from '@/lib/api/library';
import type { TacticalGeometry } from '@/lib/api/tactical';

// One geometry per map for the whole session: any analysed match of a map draws the same silhouette.
const cache = new Map<string, Promise<TacticalGeometry | null>>();

function loadGeometry(teamId: string, map: string): Promise<TacticalGeometry | null> {
  const known = cache.get(map);
  if (known !== undefined) return known;
  const pending = getSpatialOverlay({ kind: 'team', teamId, query: { map } }, 1)
    .then((response) => response.geometry)
    .catch(() => {
      cache.delete(map);
      return null;
    });
  cache.set(map, pending);
  return pending;
}

/**
 * The radar geometry of each map, read one map at a time through the team's
 * overlay with a single round. A map is absent while it loads and null when
 * none of the team's matches on it has an analysis.
 */
export function useMapGeometry(teamId: string, maps: readonly string[]): ReadonlyMap<string, TacticalGeometry | null> {
  const [geometry, setGeometry] = useState<ReadonlyMap<string, TacticalGeometry | null>>(new Map());
  const key = maps.join(',');

  useEffect(() => {
    let active = true;
    void (async () => {
      for (const map of key === '' ? [] : key.split(',')) {
        const next = await loadGeometry(teamId, map);
        if (!active) return;
        setGeometry((current) => new Map(current).set(map, next));
      }
    })();
    return () => {
      active = false;
    };
  }, [teamId, key]);

  return geometry;
}
