'use client';

import { useCallback, useEffect, useState } from 'react';
import type { TacticalSource } from '@/lib/api/tactical';
import { analysisErrorMessage, loadAnalysis } from '@/lib/replay/loaded-analysis';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { MatchAnalysisLoads } from '@/lib/replay/match-analysis-loads';
import type { MatchAnalysis as AnalysisState } from '@/lib/replay/match-analysis-loads';

export type MatchAnalysis = AnalysisState<LoadedAnalysis>;

/**
 * Tactical analyses by match, fetched the first time a match is wanted or
 * prefetched and kept for the rest of the visit. `seed` hands over one the page already loaded.
 */
export function useMatchAnalyses(
  kind: TacticalSource['kind'],
  wanted: readonly string[],
  seed?: { id: string; analysis: LoadedAnalysis },
): { analyses: ReadonlyMap<string, MatchAnalysis>; retry: (id: string) => void; prefetch: (id: string) => void } {
  const [loads] = useState(
    () =>
      new MatchAnalysisLoads<LoadedAnalysis>({
        load: (id) => loadAnalysis({ kind, id }),
        message: analysisErrorMessage,
        onChange: (states) => setAnalyses(states),
        seed,
      }),
  );
  const [analyses, setAnalyses] = useState(() => loads.states);
  const wantedKey = wanted.join(',');

  useEffect(() => {
    if (seed !== undefined) loads.seed(seed.id, seed.analysis);
  }, [loads, seed]);

  useEffect(() => {
    loads.want(wantedKey === '' ? [] : wantedKey.split(','));
  }, [loads, wantedKey]);

  const retry = useCallback((id: string) => loads.retry(id), [loads]);
  const prefetch = useCallback((id: string) => loads.prefetch(id), [loads]);
  return { analyses, retry, prefetch };
}
