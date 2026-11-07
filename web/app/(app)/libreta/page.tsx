import { Suspense, type ReactNode } from 'react';
import { LibretaHub } from '@/components/libreta/libreta-hub';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Skeleton } from '@/components/ui/skeleton';
import { LIBRETA_DESCRIPTION, LIBRETA_TITLE } from '@/lib/libreta/copy';

/** Libreta: the team's playbook, from the plays Studio reads in the demos to the calls the team agrees on. */
export default function LibretaPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <Suspense
        fallback={
          <>
            <StudioPageHeader title={LIBRETA_TITLE} description={LIBRETA_DESCRIPTION} />
            <Skeleton className="h-64 w-full" />
          </>
        }
      >
        <LibretaHub />
      </Suspense>
    </div>
  );
}
