import { Suspense, type ReactNode } from 'react';
import { VideosPageClient } from './videos-page-client';
import { Skeleton } from '@/components/ui/skeleton';

function LibraryFallback(): ReactNode {
  return (
    <div className="flex flex-col gap-8" role="status" aria-label="Cargando biblioteca">
      <Skeleton className="h-10 w-64" />
      <div className="grid grid-cols-[repeat(auto-fill,minmax(min(100%,15.5rem),1fr))] gap-5">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="aspect-[9/16] w-full" />
        ))}
      </div>
    </div>
  );
}

export default function VideosPage(): ReactNode {
  return (
    <Suspense fallback={<LibraryFallback />}>
      <VideosPageClient />
    </Suspense>
  );
}
