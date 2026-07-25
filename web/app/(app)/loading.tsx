import type { ReactElement } from 'react';
import { Skeleton } from '@/components/ui/skeleton';

/**
 * The route-level fallback. Without a `loading.tsx` boundary React never gets
 * to show one, so a cold start painted an empty rectangle between the shell and
 * the first data — which is the first impression of the product.
 *
 * Deliberately shell-shaped rather than a spinner: eyebrow, H1 block and a
 * panel grid at the geometry the real page is about to occupy, so the arriving
 * content settles into the reserved boxes instead of pushing them around.
 */
export default function AppLoading(): ReactElement {
  return (
    <div className="flex flex-col gap-8" role="status" aria-label="Cargando">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-3 w-40" />
        <Skeleton className="h-10 w-[min(100%,22rem)]" />
        <Skeleton className="h-4 w-[min(100%,34rem)]" />
      </div>
      <div className="grid gap-5 @2xl/content:grid-cols-2 @5xl/content:grid-cols-3">
        {[0, 1, 2, 3, 4, 5].map((index) => (
          <Skeleton key={index} className="h-44 w-full" />
        ))}
      </div>
    </div>
  );
}
