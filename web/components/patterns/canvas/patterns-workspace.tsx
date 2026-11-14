'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { PlugZap, Shield } from 'lucide-react';
import { useTeamAnalysis, useTeamList } from '@/hooks/use-team-analysis';
import { patternsHref } from '@/lib/analysis/pattern-canvas-mode';
import { TeamChoice } from '@/components/analysis/team-choice';
import { ImportDemosButton } from '@/components/matches/import-demos-button';
import { EmptyState } from '@/components/records/empty-state';
import { useDocumentTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PatternsCanvas } from './patterns-canvas';

/** The Buscador de patrones without a team: one sentence and the team select. */
export function PatternsIndex(): ReactNode {
  const router = useRouter();
  const teams = useTeamList();
  useDocumentTitle('Buscador de patrones · ClipHub');
  return (
    <div className="mx-auto flex w-full max-w-[59.375rem] flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="flex items-baseline gap-2 text-title font-semibold text-fg-1">
          <span aria-hidden className="text-slash">
            /
          </span>
          Buscador de patrones
        </h1>
        <ImportDemosButton />
      </div>
      <TeamChoice
        sentence="Elige el equipo del que quieres buscar granadas, posiciones de jugadores y patrones sobre el mapa."
        teams={teams}
        onPick={(teamId) => router.push(patternsHref(teamId))}
      />
    </div>
  );
}

/** The Buscador de patrones of one team: reads the team and opens the canvas. */
export function PatternsWorkspace({ teamId }: { teamId: string }): ReactNode {
  const router = useRouter();
  const { state, reload } = useTeamAnalysis(teamId);
  useDocumentTitle(state.kind === 'ready' ? `${state.data.team.name} · Buscador de patrones · ClipHub` : 'Buscador de patrones · ClipHub');
  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando el equipo" className="grid gap-2 lg:grid-cols-[18rem_minmax(0,1fr)_20rem]">
        <Skeleton className="h-96 w-full" />
        <Skeleton className="aspect-square w-full" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }
  if (state.kind === 'missing') {
    return (
      <EmptyState
        icon={Shield}
        title="Equipo no encontrado"
        description="Puede que se haya fusionado con otro equipo o que el enlace sea antiguo."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={patternsHref('')}>Elegir otro equipo</Link>
          </Button>
        }
      />
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer el equipo. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <PatternsCanvas analysis={state.data} onTeam={(next) => router.push(patternsHref(next))} onImported={reload} />;
}
