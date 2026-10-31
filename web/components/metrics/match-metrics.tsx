'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Gauge, PlugZap } from 'lucide-react';
import { LibraryApiError, getLibraryMatchMetrics, type LibraryMatch } from '@/lib/api/library';
import type { MetricsReport } from '@/lib/api/metrics';
import { playerHref } from '@/lib/clips/routes';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { MetricsView } from './metrics-view';
import { WinProbability } from './win-probability';

type State = { kind: 'loading' } | { kind: 'ready'; report: MetricsReport } | { kind: 'pending' } | { kind: 'offline' };

/** The match's advanced metrics: every player's impact, duels, trades and utility. */
export function MatchMetrics({ match }: { match: LibraryMatch }): ReactNode {
  const [state, setState] = useState<State>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    getLibraryMatchMetrics(match.id)
      .then((report) => {
        if (active) setState({ kind: 'ready', report });
      })
      .catch((error: unknown) => {
        if (!active) return;
        setState(error instanceof LibraryApiError && error.status === 409 ? { kind: 'pending' } : { kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [match.id, match.updated_at, attempt]);

  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Calculando métricas" className="flex flex-col gap-3">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'pending') {
    return (
      <EmptyState
        icon={Gauge}
        title="Análisis táctico pendiente"
        description="Las métricas se calculan con el análisis táctico de la demo. Vuelve cuando termine."
      />
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="No se pudieron calcular las métricas"
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return (
    <MetricsView
      report={state.report}
      options={{ playerHref }}
      lead={<WinProbability matchId={match.id} timelines={state.report.timelines ?? []} players={match.players ?? []} />}
    />
  );
}
