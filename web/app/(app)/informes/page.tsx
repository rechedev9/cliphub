import { Suspense, type ReactNode } from 'react';
import { ReportLibrary } from '@/components/reports/report-library';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Skeleton } from '@/components/ui/skeleton';

/** Informes: scouting reports of a rival, built on this PC from the library. */
export default function ReportsPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader
        title="Informes"
        description="Informes de scouting de un rival para el coach y el IGL, a partir de las partidas que elijas. Se generan en este PC."
      />
      <Suspense fallback={<Skeleton className="h-64 w-full" />}>
        <ReportLibrary />
      </Suspense>
    </div>
  );
}
