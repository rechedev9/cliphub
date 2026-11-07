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

// The list and a lineup page share one download per map for a while; Studio's own
// cache decides when to ask the server again, and a stale copy is never kept here.
const MAP_CACHE_MS = 10 * 60 * 1000;
const mapCache = new Map<string, { at: number; data: Promise<ProLineupMap> }>();

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
  if (cached !== undefined && !retry && Date.now() - cached.at < MAP_CACHE_MS) return cached.data;
  const next = getProLineupMap(map);
  mapCache.set(map, { at: Date.now(), data: next });
  const forget = (): void => {
    if (mapCache.get(map)?.data === next) mapCache.delete(map);
  };
  next.then((data) => {
    if (!data.fresh) forget();
  }, forget);
  return next;
}

export function useProLineupMaps(attempt: number): ProLoad<ProLineupMaps> {
  return useLoad(fetchMaps, 'maps', attempt);
}

export function useProLineupMap(map: string, attempt: number): ProLoad<ProLineupMap> {
  return useLoad(fetchMap, map, attempt);
}
