import { Suspense, type ReactNode } from 'react';
import { RankingView } from '@/components/ranking/ranking-view';
import { Skeleton } from '@/components/ui/skeleton';

/** Ranking: who plays a role best on a map, from the library's analysed demos. */
export default function RankingPage(): ReactNode {
  return (
    <div className="mx-auto flex w-full max-w-[59.5rem] flex-col gap-4">
      <h1 className="flex items-baseline gap-2 text-title font-semibold text-fg-1">
        <span aria-hidden className="text-slash">
          /
        </span>
        Ranking
      </h1>
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <RankingView />
      </Suspense>
    </div>
  );
}
