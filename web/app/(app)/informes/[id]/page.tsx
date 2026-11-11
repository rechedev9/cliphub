import type { ReactNode } from 'react';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { ReportRecord } from '@/components/reports/report-record';

/** One saved scouting report. */
export default async function ReportPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className={DOCUMENT_COLUMN}>
      <ReportRecord reportId={id} />
    </div>
  );
}
