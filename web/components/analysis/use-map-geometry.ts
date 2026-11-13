'use client';

import { useEffect, useState } from 'react';
import { getSpatialOverlay } from '@/lib/api/library';
import { GEOMETRY_SOURCE_OCCUPANCY, type TacticalGeometry } from '@/lib/api/tactical';

// One geometry per map for the whole session: any analysed match of a map draws the same silhouette.
const cache = new Map<string, Promise<TacticalGeometry | null>>();

function loadGeometry(teamId: string, map: string): Promise<TacticalGeometry | null> {
  const known = cache.get(map);
  if (known !== undefined) return known;
  const pending = getSpatialOverlay({ kind: 'team', teamId, query: { map } }, 1)
    .then((response) => {
      // Only an analysed match's geometry is the same for every team; the shipped radar has no callouts.
      if (response.geometry?.source !== GEOMETRY_SOURCE_OCCUPANCY) cache.delete(map);
      return response.geometry;
    })
    .catch(() => {
      cache.delete(map);
      return null;
    });
  cache.set(map, pending);
  return pending;
}

/**
 * The radar geometry of each map, read one map at a time through the team's
 * overlay with a single round. A map is absent while it loads; with no analysed
 * match of the team on it, it is the shipped radar, and null for an uncalibrated map.
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
