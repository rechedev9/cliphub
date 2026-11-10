'use client';

import { useCallback, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { PlugZap, Shield } from 'lucide-react';
import { TEAM_QUERY, withTeam } from '@/lib/nav-scope';
import { TeamChoice } from '@/components/analysis/team-choice';
import { EmptyState } from '@/components/records/empty-state';
import { useDocumentTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useTeamAnalysis, useTeamList } from '@/hooks/use-team-analysis';
import { TacticEditorReady } from './tactic-editor-ready';

/** Editor de tácticas: a team (`?equipo=`) opens the builder; with none chosen the page asks for one. */
export function TacticEditor(): ReactNode {
  const search = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const teams = useTeamList();
  const query = search.toString();
  const teamId = search.get(TEAM_QUERY) ?? '';
  useDocumentTitle('Editor de tácticas · ClipHub');
  const pick = useCallback((id: string) => router.replace(withTeam({ pathname, search: query }, id), { scroll: false }), [pathname, query, router]);

  if (teamId === '') {
    return (
      <TeamChoice
        sentence="El editor de tácticas parte de las rondas de un equipo en un mapa y un lado, y las condiciones que añades dejan solo las rondas que las cumplen. Elige un equipo para empezar."
        teams={teams}
        onPick={pick}
      />
    );
  }
  return <TacticEditorStage key={teamId} teamId={teamId} teams={teams} onTeam={pick} />;
}

function TacticEditorStage({ teamId, teams, onTeam }: { teamId: string; teams: ReturnType<typeof useTeamList>; onTeam: (id: string) => void }): ReactNode {
  const { state, reload } = useTeamAnalysis(teamId);
  if (state.kind === 'loading') return <Skeleton role="status" aria-label="Cargando el equipo" className="h-[60vh] w-full rounded-lg" />;
  if (state.kind === 'missing') {
    return (
      <div className="flex flex-col gap-4">
        <EmptyState icon={Shield} title="Equipo no encontrado" description="Puede que se haya fusionado con otro equipo o que el enlace sea antiguo." />
        <TeamChoice sentence="Elige otro equipo para armar su táctica." teams={teams} onPick={onTeam} />
      </div>
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
  return <TacticEditorReady analysis={state.data} teams={teams} onTeam={onTeam} />;
}
