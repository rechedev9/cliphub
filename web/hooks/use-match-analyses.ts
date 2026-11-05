'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { TacticalSource } from '@/lib/api/tactical';
import { analysisErrorMessage, loadAnalysis } from '@/lib/replay/loaded-analysis';
import type { LoadedAnalysis } from '@/lib/replay/loaded-analysis';

export type MatchAnalysis =
  | { state: 'loading' }
  | { state: 'ready'; analysis: LoadedAnalysis }
  | { state: 'failed'; message: string };

/**
 * Tactical analyses by match, fetched the first time a match is wanted and
 * kept for the rest of the visit. `seed` hands over one the page already loaded.
 */
export function useMatchAnalyses(
  kind: TacticalSource['kind'],
  wanted: readonly string[],
  seed?: { id: string; analysis: LoadedAnalysis },
): { analyses: ReadonlyMap<string, MatchAnalysis>; retry: (id: string) => void } {
  const [analyses, setAnalyses] = useState<ReadonlyMap<string, MatchAnalysis>>(() =>
    seed === undefined ? new Map() : new Map([[seed.id, { state: 'ready', analysis: seed.analysis }]]),
  );
  const started = useRef(new Set<string>(seed === undefined ? [] : [seed.id]));
  const wantedKey = wanted.join(',');
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    if (seed === undefined) return;
    started.current.add(seed.id);
    setAnalyses((current) => {
      const existing = current.get(seed.id);
      if (existing?.state === 'ready' && existing.analysis === seed.analysis) return current;
      return new Map(current).set(seed.id, { state: 'ready', analysis: seed.analysis });
    });
  }, [seed]);

  useEffect(() => {
    const ids = wantedKey === '' ? [] : wantedKey.split(',');
    for (const id of ids) {
      if (started.current.has(id)) continue;
      started.current.add(id);
      setAnalyses((current) => new Map(current).set(id, { state: 'loading' }));
      loadAnalysis({ kind, id })
        .then((analysis) => setAnalyses((current) => new Map(current).set(id, { state: 'ready', analysis })))
        .catch((error: unknown) => {
          setAnalyses((current) => new Map(current).set(id, { state: 'failed', message: analysisErrorMessage(error) }));
        });
    }
  }, [kind, wantedKey, attempt]);

  const retry = useCallback((id: string) => {
    started.current.delete(id);
    setAttempt((n) => n + 1);
  }, []);

  return { analyses, retry };
}
