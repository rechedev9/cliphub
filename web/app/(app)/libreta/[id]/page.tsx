import type { ReactNode } from 'react';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { PlayRecord } from '@/components/stratbook/play-record';

/** One saved play of the Libreta. */
export default async function PlayPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className={DOCUMENT_COLUMN}>
      <PlayRecord playId={id} />
    </div>
  );
}
