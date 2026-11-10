import { Suspense, type ReactNode } from 'react';
import { RankingView } from '@/components/ranking/ranking-view';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Skeleton } from '@/components/ui/skeleton';

/** Ranking: who plays a role best on a map, from the library's analysed demos. */
export default function RankingPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Ranking"
        description="Elige un mapa, un rol y un periodo, mira quién rinde mejor y abre sus rondas en el Replay."
      />
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <RankingView />
      </Suspense>
    </div>
  );
}
