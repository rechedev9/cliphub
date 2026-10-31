import { Suspense, type ReactNode } from 'react';
import Link from 'next/link';
import { Layers } from 'lucide-react';
import { MatchLibrary } from '@/components/matches/match-library';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { LIBRARY_SERIES_HREF } from '@/lib/clips/routes';
import { Skeleton } from '@/components/ui/skeleton';

/** Partidas: the shared demo library. A row opens the partida, its rounds, its tactics and its videos. */
export default function MatchesPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Partidas"
        description="La biblioteca común de demos: la misma partida sirve para crear vídeos y para analizarla ronda a ronda."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={LIBRARY_SERIES_HREF}>
              <Layers aria-hidden />
              Series y veto
            </Link>
          </Button>
        }
      />
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <MatchLibrary />
      </Suspense>
    </div>
  );
}
