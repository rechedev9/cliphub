import type { ReactNode } from 'react';
import { DOCUMENT_COLUMN } from '@/components/records/document-page';
import { SeriesRecord } from '@/components/series/series-record';

/** One library series. */
export default async function LibrarySeriesRecordPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className={DOCUMENT_COLUMN}>
      <SeriesRecord seriesId={id} />
    </div>
  );
}
