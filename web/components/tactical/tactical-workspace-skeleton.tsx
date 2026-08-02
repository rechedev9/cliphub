import type { ReactNode } from 'react';
import { Skeleton } from '@/components/ui/skeleton';

/**
 * The workspace's shape while it loads. It reserves the real geometry — header,
 * filter bar, round column, square radar — so nothing jumps when the document
 * and the position blob land.
 */
export function TacticalWorkspaceSkeleton(): ReactNode {
  return (
    <div className="flex flex-col gap-8 sm:gap-10" aria-hidden>
      <div className="flex flex-col gap-3">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-5 w-96 max-w-full" />
      </div>
      <Skeleton className="h-[104px] w-full rounded-lg" />
      <div className="grid gap-6 sm:gap-8 xl:grid-cols-[minmax(0,360px)_minmax(0,1fr)]">
        <Skeleton className="h-[520px] w-full rounded-lg" />
        <Skeleton className="h-[520px] w-full rounded-lg" />
      </div>
    </div>
  );
}
