import type { ReactNode } from 'react';
import { ReportRecord } from '@/components/reports/report-record';

/** One saved scouting report. */
export default async function ReportPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <ReportRecord reportId={id} />
    </div>
  );
}
