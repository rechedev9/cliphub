'use client';

import { useEffect, useState } from 'react';
import { getBaseline, getLeaderboard } from '@/lib/api/library';
import { BASELINE_METRICS, type Baseline, type BaselineMetric, type Leaderboard } from '@/lib/api/baseline';
import { adaptiveMinRounds } from '@/lib/analysis/library-pool';

/** The library and pro references of a map at the library's adaptive minimum, with every player's line per metric. */
export type References = {
  readonly library: Baseline;
  readonly pro: Baseline;
  readonly minRounds: number;
  /** Every player with a round, per metric asked for; `kpr` is always there. */
  readonly boards: ReadonlyMap<BaselineMetric, Leaderboard>;
};

/** The leaderboard is the only per-player source; its cap bounds the lookup. */
const LOOKUP_LIMIT = 500;
const FRESH_MS = 60_000;
const cache = new Map<string, { at: number; pending: Promise<References> }>();

async function loadReferences(map: string, metrics: readonly BaselineMetric[]): Promise<References> {
  const wanted = [...new Set<BaselineMetric>(['kpr', ...metrics])];
  const boards = await Promise.all(
    wanted.map((metric) => getLeaderboard({ metric, scope: 'library', map, min_rounds: 1, limit: LOOKUP_LIMIT })),
  );
  const minRounds = adaptiveMinRounds(boards[0]?.players.map((row) => row.rounds) ?? []);
  const [library, pro] = await Promise.all([
    getBaseline({ scope: 'library', map, min_rounds: minRounds }),
    getBaseline({ scope: 'pro', map, min_rounds: minRounds }),
  ]);
  return { library, pro, minRounds, boards: new Map(wanted.flatMap((metric, index) => {
      const board = boards[index];
      return board === undefined ? [] : [[metric, board] as const];
    })) };
}

function cached(map: string, metrics: readonly BaselineMetric[]): Promise<References> {
  const key = `${map}|${[...metrics].sort().join(',')}`;
  const hit = cache.get(key);
  if (hit && Date.now() - hit.at < FRESH_MS) return hit.pending;
  const pending = loadReferences(map, metrics).catch((error: unknown) => {
    cache.delete(key);
    throw error;
  });
  cache.set(key, { at: Date.now(), pending });
  return pending;
}

/** The library baseline of each map ("" for every map) at its adaptive minimum; null while loading, an empty map on failure. */
export function useLibraryBaselines(maps: readonly string[]): ReadonlyMap<string, Baseline> | null {
  const [state, setState] = useState<{ key: string; value: ReadonlyMap<string, Baseline> } | null>(null);
  const key = [...new Set(maps)].sort().join(',');
  useEffect(() => {
    let active = true;
    const wanted = key.split(',');
    Promise.all(wanted.map((map) => cached(map, []).then((refs): [string, Baseline][] => [[map, refs.library]]).catch((): [string, Baseline][] => [])))
      .then((entries) => {
        if (active) setState({ key, value: new Map(entries.flat()) });
      })
      .catch(() => {
        if (active) setState({ key, value: new Map() });
      });
    return () => {
      active = false;
    };
  }, [key]);
  return state?.key === key ? state.value : null;
}

/** The references of a map, shared by the context bar and the view under it; null while loading. */
export function useReferences(map: string, metrics: readonly BaselineMetric[] = []): References | null | 'failed' {
  const [state, setState] = useState<References | null | 'failed'>(null);
  const key = metrics.join(',');
  useEffect(() => {
    let active = true;
    setState(null);
    cached(map, BASELINE_METRICS.filter((metric) => key.split(',').includes(metric)))
      .then((next) => {
        if (active) setState(next);
      })
      .catch(() => {
        if (active) setState('failed');
      });
    return () => {
      active = false;
    };
  }, [map, key]);
  return state;
}
