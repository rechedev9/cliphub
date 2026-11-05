import type { ReactNode } from 'react';
import { Skeleton } from '@/components/ui/skeleton';

/**
 * The workspace's shape while it loads: the summary, then the replay's three
 * columns and its strip, so nothing jumps when the analysis lands.
 */
export function TacticalWorkspaceSkeleton(): ReactNode {
  return (
    <div className="flex flex-col gap-6 sm:gap-8" aria-hidden>
      <div className="flex flex-col gap-3">
        <Skeleton className="h-4 w-40" />
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-5 w-96 max-w-full" />
      </div>
      <Skeleton className="h-9 w-64" />
      <div className="grid gap-5 lg:grid-cols-[230px_minmax(0,1fr)_270px]">
        <Skeleton className="h-[480px] w-full rounded-lg" />
        <Skeleton className="h-[480px] w-full rounded-lg" />
        <Skeleton className="h-[480px] w-full rounded-lg" />
      </div>
      <Skeleton className="h-[180px] w-full rounded-lg" />
    </div>
  );
}
