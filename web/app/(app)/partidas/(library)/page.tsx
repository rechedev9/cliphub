import { Suspense, type ReactNode } from 'react';
import { MatchLibrary, PartidasSkeleton } from '@/components/matches/match-library';

/** Partidas > Privadas: the shared demo library. A row opens its 2D replay; its record holds the rounds, tactics and videos. */
export default function MatchesPage(): ReactNode {
  return (
    <Suspense fallback={<PartidasSkeleton />}>
      <MatchLibrary />
    </Suspense>
  );
}
