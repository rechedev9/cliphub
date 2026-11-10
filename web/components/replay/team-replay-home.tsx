'use client';

import { useCallback, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { TEAM_QUERY, withTeam } from '@/lib/nav-scope';
import { TeamChoice } from '@/components/analysis/team-choice';
import { useTeamList } from '@/hooks/use-team-analysis';
import { ReplayHome } from './replay-home';
import { TeamReplayStage } from './team-replay-stage';

/**
 * Replay 2D opened on its own: a team (`?equipo=`) opens its rounds with the filter stack over the map;
 * with none chosen the page asks for one and keeps the latest partidas below it.
 */
export function TeamReplayHome(): ReactNode {
  const search = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const teams = useTeamList();
  const teamId = search.get(TEAM_QUERY) ?? '';
  const query = search.toString();
  const pick = useCallback((id: string) => router.replace(withTeam({ pathname, search: query }, id), { scroll: false }), [pathname, query, router]);

  if (teamId !== '') return <TeamReplayStage key={teamId} teamId={teamId} />;
  return (
    <div className="flex flex-col gap-8">
      <TeamChoice
        sentence="El replay 2D de un equipo reúne sus rondas de varias partidas, filtradas por mapa, lado y economía. Elige un equipo para empezar."
        teams={teams}
        onPick={pick}
      />
      <ReplayHome />
    </div>
  );
}
