import { Suspense, type ReactNode } from 'react';
import { MatchLibrary, PartidasHeader } from '@/components/matches/match-library';
import { Skeleton } from '@/components/ui/skeleton';

/** Partidas: the shared demo library. A row opens the partida, its rounds, its tactics and its videos. */
export default function MatchesPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <Suspense
        fallback={
          <>
            <PartidasHeader importer={<Skeleton className="h-10 w-40" />} />
            <Skeleton className="h-64 w-full" />
          </>
        }
      >
        <MatchLibrary />
      </Suspense>
    </div>
  );
}
