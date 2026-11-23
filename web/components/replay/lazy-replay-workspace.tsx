'use client';

import type { ReactNode } from 'react';
import dynamic from 'next/dynamic';
import { Skeleton } from '@/components/ui/skeleton';

function ReplayWorkspaceLoading(): ReactNode {
  return (
    <div role="status" aria-label="Cargando el Replay" className="flex flex-col gap-5">
      <div className="grid gap-5 lg:grid-cols-[230px_minmax(0,1fr)_270px]">
        <Skeleton className="h-[480px] w-full rounded-lg" />
        <Skeleton className="h-[480px] w-full rounded-lg" />
        <Skeleton className="h-[480px] w-full rounded-lg" />
      </div>
      <Skeleton className="h-[180px] w-full rounded-lg" />
    </div>
  );
}

export const LazyReplayWorkspace = dynamic(() => import('./replay-workspace').then((loaded) => loaded.ReplayWorkspace), {
  loading: ReplayWorkspaceLoading,
});
