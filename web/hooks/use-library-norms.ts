'use client';

import { useEffect, useState } from 'react';
import { getLibraryNorms } from '@/lib/api/library';
import type { LibraryNorms } from '@/lib/api/norms';

const FRESH_MS = 60_000;
const cache = new Map<string, { at: number; pending: Promise<LibraryNorms> }>();

function cached(map: string): Promise<LibraryNorms> {
  const hit = cache.get(map);
  if (hit && Date.now() - hit.at < FRESH_MS) return hit.pending;
  const pending = getLibraryNorms({ map }).catch((error: unknown) => {
    cache.delete(map);
    throw error;
  });
  cache.set(map, { at: Date.now(), pending });
  return pending;
}

/**
 * The library's team rates on a map (every map for ""), over all its dates:
 * the reference a team table compares with. Null while loading.
 */
export function useLibraryNorms(map: string): LibraryNorms | null | 'failed' {
  const [state, setState] = useState<{ map: string; value: LibraryNorms | 'failed' } | null>(null);
  useEffect(() => {
    let active = true;
    cached(map)
      .then((value) => {
        if (active) setState({ map, value });
      })
      .catch(() => {
        if (active) setState({ map, value: 'failed' });
      });
    return () => {
      active = false;
    };
  }, [map]);
  return state?.map === map ? state.value : null;
}
