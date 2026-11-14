'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { TacticalSource } from '@/lib/api/tactical';
import { analysisErrorMessage, loadAnalysis } from '@/lib/replay/loaded-analysis';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';
import { MatchAnalysisLoads } from '@/lib/replay/match-analysis-loads';
import type { AnalysisLoad } from '@/lib/replay/match-analysis-loads';

export type MatchAnalysis = AnalysisLoad<LoadedAnalysis>;

/** `MatchAnalysisLoads` for React: `wanted` loads now, `prefetch` before the match is wanted, `seed` is one the page already has. */
export function useMatchAnalyses(
  kind: TacticalSource['kind'],
  wanted: readonly string[],
  seed?: { id: string; analysis: LoadedAnalysis },
): { analyses: ReadonlyMap<string, MatchAnalysis>; retry: (id: string) => void; prefetch: (id: string) => void } {
  const kindRef = useRef(kind);
  useEffect(() => {
    kindRef.current = kind;
  }, [kind]);
  const [loads] = useState(
    () =>
      new MatchAnalysisLoads<LoadedAnalysis>({
        load: (id) => loadAnalysis({ kind: kindRef.current, id }),
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
  const prefetch = useCallback((id: string) => loads.want([id]), [loads]);
  return { analyses, retry, prefetch };
}
