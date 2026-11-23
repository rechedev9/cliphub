'use client';

import { useCallback, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { TEAM_QUERY, withTeam } from '@/lib/nav-scope';
import { TeamChoice } from '@/components/analysis/team-choice';
import { useDocumentTitle } from '@/components/shell/route-title';
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

  useDocumentTitle(teamId === '' ? 'Replay 2D · ClipHub' : undefined);
  if (teamId !== '') return <TeamReplayStage key={teamId} teamId={teamId} />;
  return (
    <div className="flex flex-col gap-6">
      <h1 className="flex items-baseline gap-2 text-title font-semibold text-fg-1">
        <span aria-hidden className="text-slash">
          /
        </span>
        Replay 2D
      </h1>
      <section aria-labelledby="replay-team-title" className="flex flex-col gap-2">
        <h2 id="replay-team-title" className="text-body font-semibold text-fg-1">
          Rondas de un equipo
        </h2>
        <TeamChoice
          sentence="Junta las rondas de un equipo de varias partidas y fíltralas por mapa, lado y economía."
          teams={teams}
          onPick={pick}
        />
      </section>
      <ReplayHome />
    </div>
  );
}
