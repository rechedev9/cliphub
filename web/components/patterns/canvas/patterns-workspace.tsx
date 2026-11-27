'use client';

import { useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Shield } from 'lucide-react';
import { useTeamAnalysis, useTeamList } from '@/hooks/use-team-analysis';
import { usePublicTeamSync, usePublicTeams } from '@/hooks/use-public-team';
import { importProTeam, type ProTeamCount } from '@/lib/api/pro-matches';
import { PUBLIC_TEAM_DAYS, publicOnlyTeams, syncPublicTeam } from '@/lib/analysis/public-team';
import { patternsHref } from '@/lib/analysis/pattern-canvas-mode';
import { TeamChoice } from '@/components/analysis/team-choice';
import { ImportDemosButton } from '@/components/matches/import-demos-button';
import { EmptyState } from '@/components/records/empty-state';
import { LoadErrorState } from '@/components/records/load-error-state';
import { useDocumentTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PatternsCanvas } from './patterns-canvas';

/** The Buscador de patrones without a team: one sentence and the team select. */
export function PatternsIndex(): ReactNode {
  const router = useRouter();
  const teams = useTeamList();
  const publicTeams = usePublicTeams();
  const offered = useMemo(() => publicOnlyTeams(teams ?? [], publicTeams ?? []), [teams, publicTeams]);
  const [opening, setOpening] = useState<{ name: string; failed: boolean } | null>(null);
  useDocumentTitle('Buscador de patrones · ClipHub');
  const openPublic = (team: ProTeamCount): void => {
    setOpening({ name: team.name, failed: false });
    syncPublicTeam({ hltvId: team.id, call: importProTeam, onBatch: (progress) => progress.teamId === null })
      .then((progress) => {
        if (progress.teamId === null) setOpening({ name: team.name, failed: true });
        else router.push(patternsHref(progress.teamId));
      })
      .catch(() => setOpening({ name: team.name, failed: true }));
  };
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
        sentence={`Elige el equipo del que quieres buscar granadas, posiciones de jugadores y patrones sobre el mapa. La búsqueda junta tus demos con todas sus partidas públicas de los últimos ${PUBLIC_TEAM_DAYS} días.`}
        teams={teams}
        onPick={(teamId) => router.push(patternsHref(teamId))}
        publicTeams={offered}
        onPickPublic={openPublic}
        disabled={opening !== null && !opening.failed}
      />
      {opening === null ? null : (
        <p role="status" className="text-body-sm text-fg-2">
          {opening.failed ? `No se pudieron traer las partidas públicas de ${opening.name}. Comprueba la conexión y vuelve a elegirlo.` : `Trayendo las partidas públicas de ${opening.name}…`}
        </p>
      )}
    </div>
  );
}

/** The Buscador de patrones of one team: reads the team and opens the canvas. */
export function PatternsWorkspace({ teamId }: { teamId: string }): ReactNode {
  const router = useRouter();
  const { state, reload } = useTeamAnalysis(teamId);
  const publicSync = usePublicTeamSync(teamId, reload);
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
  if (state.kind === 'failed') {
    return (
      <LoadErrorState error={state.error} title="No se pudo leer el equipo" onRetry={reload} />
    );
  }
  return <PatternsCanvas analysis={state.data} onTeam={(next) => router.push(patternsHref(next))} onImported={reload} publicSync={publicSync} />;
}
