import { Suspense, type ReactNode } from 'react';
import { MatchLibrary, PartidasSkeleton } from '@/components/matches/match-library';

/** Partidas: the shared demo library. A row opens the partida, its rounds, its tactics and its videos. */
export default function MatchesPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <Suspense fallback={<PartidasSkeleton />}>
        <MatchLibrary />
      </Suspense>
    </div>
  );
}
