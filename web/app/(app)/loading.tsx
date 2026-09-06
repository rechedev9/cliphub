import type { ReactElement } from 'react';
import { Skeleton } from '@/components/ui/skeleton';

export default function AppLoading(): ReactElement {
  return (
    <div className="measure-list flex flex-col gap-6" role="status" aria-label="Cargando">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-10 w-64" />
        <Skeleton className="measure-read h-5 w-full" />
      </div>
      <div className="grid gap-3 @[48rem]/content:grid-cols-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-36 w-full" />
        ))}
      </div>
      <div className="flex flex-col gap-3">
        {Array.from({ length: 3 }).map((_, index) => (
          <Skeleton key={index} className="h-[85px] w-full" />
        ))}
      </div>
    </div>
  );
}
