'use client';

import { useEffect, useState } from 'react';
import { isLibraryOffline, LibraryApiError } from '@/lib/api/library';
import { getProLineupMap, listProLineupMaps, type ProLineupMap, type ProLineupMaps } from '@/lib/api/pro-lineups';

export type ProLoad<T> =
  | { kind: 'loading' }
  | { kind: 'ready'; data: T }
  | { kind: 'missing' }
  | { kind: 'offline' }
  | { kind: 'server_down' }
  | { kind: 'failed' };

// One download per map while Studio stays open: the list and a lineup page share it.
const mapCache = new Map<string, Promise<ProLineupMap>>();

function failure<T>(error: unknown): ProLoad<T> {
  if (isLibraryOffline(error)) return { kind: 'offline' };
  if (error instanceof LibraryApiError && error.status === 404) return { kind: 'missing' };
  if (error instanceof LibraryApiError && (error.code === 'community_unavailable' || error.code === 'community_disabled')) {
    return { kind: 'server_down' };
  }
  return { kind: 'failed' };
}

// Fetchers live at module level so the effect depends only on the key and the attempt.
function useLoad<T>(fetcher: (key: string, retry: boolean) => Promise<T>, key: string, attempt: number): ProLoad<T> {
  const [state, setState] = useState<ProLoad<T>>({ kind: 'loading' });
  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    fetcher(key, attempt > 0)
      .then((data) => {
        if (active) setState({ kind: 'ready', data });
      })
      .catch((error: unknown) => {
        if (active) setState(failure<T>(error));
      });
    return () => {
      active = false;
    };
  }, [fetcher, key, attempt]);
  return state;
}

function fetchMaps(): Promise<ProLineupMaps> {
  return listProLineupMaps();
}

function fetchMap(map: string, retry: boolean): Promise<ProLineupMap> {
  const cached = mapCache.get(map);
  if (cached !== undefined && !retry) return cached;
  const next = getProLineupMap(map);
  mapCache.set(map, next);
  next.catch(() => mapCache.delete(map));
  return next;
}

export function useProLineupMaps(attempt: number): ProLoad<ProLineupMaps> {
  return useLoad(fetchMaps, 'maps', attempt);
}

export function useProLineupMap(map: string, attempt: number): ProLoad<ProLineupMap> {
  return useLoad(fetchMap, map, attempt);
}
