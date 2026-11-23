'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { ChevronRight, Download, ExternalLink, History, UploadCloud } from 'lucide-react';
import { listFaceitMatches, type FaceitMatch } from '@/lib/api/faceit';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { MatchHistory } from './match-history';

const MATCH_TO_CLIP = [
  { icon: ExternalLink, text: 'Abre la sala de una partida' },
  { icon: Download, text: 'Descarga su demo' },
  { icon: UploadCloud, text: 'Súbela con «Subir demo descargada»' },
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
    body = <div role="status" aria-label="Cargando partidas"><Skeleton className="h-96 w-full" /></div>;
  } else if (error && matches === null) {
    body = <EmptyState icon={History} title="No se pudieron cargar las partidas"
      description="Vuelve a intentarlo para consultar el historial de FACEIT."
      actions={<Button variant="outline" onClick={refresh}>Reintentar</Button>} />;
  } else if (matches?.length === 0) {
    body = <EmptyState icon={History} title="Sin partidas recientes"
      description="Cuando este jugador termine una partida en FACEIT aparecerá aquí."
      actions={<Button variant="outline" onClick={refresh} loading={loading} loadingText="Actualizando…">Actualizar partidas</Button>} />;
  } else {
    body = <MatchHistory matches={matches ?? []} refreshing={loading} onRefresh={refresh} />;
  }

  return (
    <div className="flex min-w-0 flex-col gap-5 p-4 sm:p-5">
      {error && matches !== null ? <p role="alert" className="text-body-sm text-destructive">No se pudo actualizar el historial. Vuelve a intentarlo con «Actualizar partidas».</p> : null}
      {/* The three manual steps sit above the table: ClipHub does not fetch FACEIT demos by itself. */}
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
      {body}
    </div>
  );
}
