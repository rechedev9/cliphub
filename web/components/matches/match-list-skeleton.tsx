import { Skeleton } from '@/components/ui/skeleton';

/**
 * Loading placeholder for the scoreboard. Every box mirrors `MatchRow`'s real
 * geometry — the same panel, padding, gaps, container-query grid and control
 * heights — so the list does not visibly re-lay-out when the data lands. The
 * version this replaces used `gap-6`/`gap-7`, a `w-[150px]` title block, a 3px
 * accent bar against the row's 4px one and an `h-9` stand-in for an `h-11`
 * button; at 390px its stat strip alone pushed 111px of horizontal overflow.
 *
 * It renders the *unscored* template on purpose: the local pipeline computes no
 * round score, so an empty score lane is the shape the data actually arrives in.
 */
export function MatchListSkeleton() {
  return (
    <div role="status" aria-busy="true" aria-label="Cargando demos" className="flex flex-col gap-3">
      {Array.from({ length: 4 }).map((_, i) => (
        <div key={i} className="studio-panel px-4 py-4 @[34rem]/content:px-5">
          <div className="flex items-stretch gap-4 @[34rem]/content:gap-5">
            <Skeleton className="w-1 shrink-0 rounded-none" />
            <Skeleton className="hidden aspect-video w-[5.5rem] shrink-0 self-center rounded-none @[42rem]/content:block" />

            <div className="grid min-w-0 flex-1 grid-cols-1 items-center gap-x-4 gap-y-3 @[56rem]/content:grid-cols-[minmax(0,1.2fr)_auto_auto] @[56rem]/content:gap-x-6">
              <div className="min-w-0">
                <Skeleton className="h-6 w-32" />
                <Skeleton className="mt-1 h-4 w-48 max-w-full" />
              </div>

              <div className="col-span-full grid grid-cols-5 gap-3 border-y border-border-subtle py-3 @[34rem]/content:gap-5 @[56rem]/content:col-span-1 @[56rem]/content:flex @[56rem]/content:items-center @[56rem]/content:gap-6 @[56rem]/content:border-0 @[56rem]/content:py-0">
                {Array.from({ length: 5 }).map((__, j) => (
                  <div key={j} className="flex min-w-0 flex-col gap-1">
                    <Skeleton className="h-4 w-6" />
                    <Skeleton className="h-7 w-10 max-w-full" />
                  </div>
                ))}
              </div>

              <div className="col-span-full flex items-center justify-end gap-2 @[56rem]/content:col-span-1">
                <Skeleton className="h-11 w-full max-w-40 @[34rem]/content:w-36" />
                <Skeleton className="size-11 shrink-0" />
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  );
}
