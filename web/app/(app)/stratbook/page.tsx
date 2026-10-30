import { Suspense, type ReactNode } from 'react';
import { StratbookLibrary } from '@/components/stratbook/stratbook-library';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Skeleton } from '@/components/ui/skeleton';

/** Stratbook: saved plays, your own and the rival's, each tied to its example rounds. */
export default function StratbookPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Stratbook"
        description="Las jugadas que quieres recordar, propias o del rival, con las rondas de las que salen. Se guardan en este PC y se comparten como fichero."
      />
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <StratbookLibrary />
      </Suspense>
    </div>
  );
}
