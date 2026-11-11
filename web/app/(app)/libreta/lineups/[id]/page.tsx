import type { ReactNode } from 'react';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { LineupRecord } from '@/components/libreta/lineup-record';

/** One saved lineup of the Libreta. */
export default async function LineupPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className={DOCUMENT_COLUMN}>
      <LineupRecord lineupId={id} />
    </div>
  );
}
