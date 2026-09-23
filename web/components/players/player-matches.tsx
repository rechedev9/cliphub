'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { ChevronRight, Download, ExternalLink, History, UploadCloud } from 'lucide-react';
import { listFaceitMatches, type FaceitMatch } from '@/lib/api/faceit';
import { StudioEmptyState } from '@/components/studio/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { MatchHistory } from './match-history';
import { PlayerPerformance } from './player-performance';

const MATCH_TO_CLIP = [
  { icon: ExternalLink, text: 'Abre la sala en FACEIT' },
  { icon: Download, text: 'Descarga la demo' },
  { icon: UploadCloud, text: 'Súbela a ClipHub' },
] as const;

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
      {/* The page's whole purpose as three scannable steps, instead of a sentence in a raised card under the table. */}
      <aside aria-labelledby="match-to-clip-heading" className="flex flex-col gap-3 rounded-lg border border-border-subtle bg-surface-1 p-4 @[56rem]/content:flex-row @[56rem]/content:items-center @[56rem]/content:gap-6">
        <h3 id="match-to-clip-heading" className="shrink-0 text-body-sm font-semibold text-fg-1">De la partida al clip</h3>
        <ol className="flex min-w-0 flex-col gap-2 @[40rem]/content:flex-row @[40rem]/content:items-center @[40rem]/content:gap-3">
          {MATCH_TO_CLIP.map(({ icon: Icon, text }, index) => (
            <li key={text} className="flex items-center gap-3 text-body-sm text-fg-2">
              {index > 0 ? <ChevronRight aria-hidden className="hidden size-4 shrink-0 text-fg-4 @[40rem]/content:block" /> : null}
              <span className="grid size-8 shrink-0 place-items-center rounded-md border border-border bg-surface-2"><Icon aria-hidden className="size-4" /></span>
              {text}
            </li>
          ))}
        </ol>
      </aside>
    </div>
  );
}
