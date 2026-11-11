import { Suspense, type ReactNode } from 'react';
import { ReportLibrary } from '@/components/reports/report-library';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { Skeleton } from '@/components/ui/skeleton';

/** Informes: scouting reports of a rival, built on this PC from the library. */
export default function ReportsPage(): ReactNode {
  return (
    <Suspense fallback={<Skeleton className={`${DOCUMENT_COLUMN} h-64`} />}>
      <ReportLibrary />
    </Suspense>
  );
}
