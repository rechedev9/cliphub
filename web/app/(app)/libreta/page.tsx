import { Suspense, type ReactNode } from 'react';
import { LibretaHub } from '@/components/libreta/libreta-hub';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Skeleton } from '@/components/ui/skeleton';

/** Libreta: the team's playbook, from the plays Studio reads in the demos to the calls the team agrees on. */
export default function LibretaPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Libreta"
        description="El plan de tu equipo: jugadas, lineups, roles, llamadas y callouts. Se guarda en este PC y se comparte como fichero."
      />
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <LibretaHub />
      </Suspense>
    </div>
  );
}
