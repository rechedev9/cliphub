'use client';

import type { ReactNode } from 'react';
import { Radar, RefreshCw } from 'lucide-react';
import type { MatchAnalysis } from '@/hooks/use-match-analyses';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

/** What the map shows while the round's match loads, failed, or does not hold the round. */
export function CentreState({
  state,
  round,
  onRetry,
}: {
  state: MatchAnalysis | undefined;
  round: number | undefined;
  onRetry: () => void;
}): ReactNode {
  if (state === undefined || state.state === 'loading') {
    return <Skeleton role="status" aria-label="Cargando la partida" className="absolute inset-3" />;
  }
  if (state.state === 'failed') {
    return (
      <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 px-6 text-center">
        <p className="text-body-sm text-fg-2">{state.message}</p>
        <Button type="button" variant="outline" size="sm" onClick={onRetry}>
          <RefreshCw aria-hidden />
          Reintentar
        </Button>
      </div>
    );
  }
  return (
    <div className="absolute inset-0 flex flex-col items-center justify-center gap-3 px-6 text-center">
      <Radar className="size-6 text-fg-3" aria-hidden />
      <p className="text-body-sm text-fg-2">La ronda {round} no está en el análisis de esta partida.</p>
    </div>
  );
}
