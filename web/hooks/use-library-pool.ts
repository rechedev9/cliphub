'use client';

import { useEffect, useState } from 'react';
import { listLibraryTeams } from '@/lib/api/library';

/** Teams read for a library reference; a bigger library pools its most active teams. */
const MAX_TEAMS = 16;

/**
 * One answer per library team for `key`: the input of a library reference for
 * a team table until the backend serves one. `fetchOne` must be stable (a
 * module function or a memoised callback). A team that fails to load is left
 * out; null while reading.
 */
export function useLibraryPool<T>(key: string, fetchOne: (teamId: string) => Promise<T>): T[] | null {
  const [state, setState] = useState<T[] | null>(null);
  useEffect(() => {
    let active = true;
    setState(null);
    listLibraryTeams()
      .then((teams) => {
        const chosen = [...teams].sort((a, b) => b.match_count - a.match_count).slice(0, MAX_TEAMS);
        return Promise.all(chosen.map((team) => fetchOne(team.id).then((value): T[] => [value]).catch((): T[] => [])));
      })
      .then((results) => {
        if (active) setState(results.flat());
      })
      .catch(() => {
        if (active) setState([]);
      });
    return () => {
      active = false;
    };
  }, [key, fetchOne]);
  return state;
}
