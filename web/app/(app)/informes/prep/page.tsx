import { Suspense, type ReactNode } from 'react';
import { PrepRecord } from '@/components/prep/prep-record';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { Skeleton } from '@/components/ui/skeleton';

/** The printable prep of a rival on one map: buys, tempo, CT positions and playbook. */
export default function PrepPage(): ReactNode {
  return (
    <div className={`${DOCUMENT_COLUMN} print:max-w-none`}>
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <PrepRecord />
      </Suspense>
    </div>
  );
}
