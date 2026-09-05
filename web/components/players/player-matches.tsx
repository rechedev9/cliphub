'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { History, UploadCloud } from 'lucide-react';
import { listFaceitMatches, type FaceitMatch } from '@/lib/api/faceit';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { MatchHistory } from './match-history';
import { PlayerPerformance } from './player-performance';

export function PlayerMatches({ playerID, enabled }: { playerID: string; enabled: boolean }): ReactNode {
  const [matches, setMatches] = useState<FaceitMatch[] | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [revision, setRevision] = useState(0);
  const refresh = (): void => setRevision((value) => value + 1);

  useEffect(() => {
    if (!enabled) return;
    let cancelled = false;
    setLoading(true);
    setError(false);
    void listFaceitMatches(playerID, 20).then((rows) => {
      if (!cancelled) setMatches(rows);
    }).catch(() => {
      if (!cancelled) setError(true);
    }).finally(() => {
      if (!cancelled) setLoading(false);
    });
    return () => { cancelled = true; };
  }, [playerID, enabled, revision]);

  let body: ReactNode;
  if (!enabled) {
    body = <p className="text-body-sm text-fg-2">El historial estará disponible cuando vuelva la conexión.</p>;
  } else if (matches === null && loading) {
    body = <div role="status" aria-label="Cargando partidas" className="space-y-5">
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">{[0, 1, 2, 3].map((slot) => <Skeleton key={slot} className="h-28" />)}</div>
      <Skeleton className="h-96 w-full" />
    </div>;
  } else if (error && matches === null) {
    body = <StudioEmptyState icon={History} title="No se pudieron cargar las partidas"
      description="Vuelve a intentarlo para consultar el historial de FACEIT."
      actions={<Button variant="outline" onClick={refresh}>Reintentar</Button>} compact />;
  } else if (matches?.length === 0) {
    body = <StudioEmptyState icon={History} title="Sin partidas recientes"
      description="Cuando este jugador termine una partida en FACEIT aparecerá aquí."
      actions={<Button variant="outline" onClick={refresh} loading={loading} loadingText="Actualizando…">Actualizar partidas</Button>} compact />;
  } else {
    body = <>
      <PlayerPerformance matches={matches ?? []} />
      <MatchHistory matches={matches ?? []} refreshing={loading} onRefresh={refresh} />
    </>;
  }

  return (
    <div className="flex min-w-0 flex-col gap-5 p-4 sm:p-5">
      {error && matches !== null ? <p role="alert" className="text-body-sm text-destructive">No se pudo actualizar el historial. Vuelve a intentarlo con «Actualizar partidas».</p> : null}
      {body}
      <aside className="flex items-center gap-4 rounded-lg border border-border bg-surface-3 p-4">
        <UploadCloud aria-hidden className="size-6 shrink-0 text-fg-2" />
        <div><h3 className="text-body font-semibold text-fg-1">De la partida al clip</h3>
          <p className="mt-1 text-body-sm text-fg-2">Abre la sala en FACEIT, descarga la demo y súbela a ClipHub.</p>
        </div>
      </aside>
    </div>
  );
}
