import { Suspense, type ReactNode } from 'react';
import { TacticEditor } from '@/components/tactics/tactic-editor';
import { Skeleton } from '@/components/ui/skeleton';

/** Editor de tácticas: the filter-panel builder that saves into the Libreta as a play. */
export default function NewTacticPage(): ReactNode {
  return (
    <Suspense fallback={<Skeleton className="h-[60vh] w-full rounded-lg" />}>
      <TacticEditor />
    </Suspense>
  );
}
