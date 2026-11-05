'use client';

import { useEffect, useRef, useState } from 'react';
import { LibraryApiError, getLibraryMatchMetrics } from '@/lib/api/library';
import type { RoundTimeline as WinTimeline } from '@/lib/api/metrics';

export type WinTimelines = { kind: 'loading' } | { kind: 'ready'; timelines: readonly WinTimeline[] } | { kind: 'none'; reason: string };

/** The win-probability timelines of a library match, read once per match and visit. */
export function useWinTimelines(matchId: string | null): WinTimelines | null {
  const [byMatch, setByMatch] = useState<ReadonlyMap<string, WinTimelines>>(new Map());
  const started = useRef(new Set<string>());

  useEffect(() => {
    if (matchId === null || started.current.has(matchId)) return;
    started.current.add(matchId);
    setByMatch((current) => new Map(current).set(matchId, { kind: 'loading' }));
    getLibraryMatchMetrics(matchId)
      .then((report) => setByMatch((current) => new Map(current).set(matchId, { kind: 'ready', timelines: report.timelines ?? [] })))
      .catch((error: unknown) => {
        const reason = error instanceof LibraryApiError && error.status === 409
          ? 'La probabilidad de victoria aparece cuando termina el análisis táctico.'
          : 'No se pudo calcular la probabilidad de victoria de esta partida.';
        setByMatch((current) => new Map(current).set(matchId, { kind: 'none', reason }));
      });
  }, [matchId]);

  return matchId === null ? null : (byMatch.get(matchId) ?? { kind: 'loading' });
}
