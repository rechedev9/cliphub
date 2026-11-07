'use client';

import { useEffect, useState } from 'react';
import { fetchTacticalDocument, isServiceUnavailableError, isTacticalNotReadyError } from '@/lib/api/tactical';
import { roundRows, type RoundRow } from '@/lib/library/rounds';

export type MatchRoundRows =
  | { readonly kind: 'loading' }
  /** The tactical document is not written yet: the scan is still running. */
  | { readonly kind: 'pending' }
  | { readonly kind: 'error'; readonly message: string; readonly retry: () => void }
  | { readonly kind: 'ready'; readonly rows: readonly RoundRow[] };

/** The rounds of a library partida, read from its tactical document as the Rondas tab reads them. */
export function useMatchRoundRows(matchId: string): MatchRoundRows {
  const [state, setState] = useState<MatchRoundRows>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    fetchTacticalDocument({ kind: 'library', id: matchId })
      .then((doc) => {
        if (active) setState({ kind: 'ready', rows: roundRows(doc) });
      })
      .catch((err: unknown) => {
        if (!active) return;
        if (isTacticalNotReadyError(err)) {
          setState({ kind: 'pending' });
          return;
        }
        setState({
          kind: 'error',
          message: isServiceUnavailableError(err)
            ? 'El servicio local no está disponible.'
            : 'No se pudo leer el documento táctico de esta partida.',
          retry: () => setAttempt((n) => n + 1),
        });
      });
    return () => {
      active = false;
    };
  }, [matchId, attempt]);

  return state;
}
