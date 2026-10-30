import type { ReactNode } from 'react';
import { SeriesRecord } from '@/components/series/series-record';

/** One library series. */
export default async function LibrarySeriesRecordPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <SeriesRecord seriesId={id} />
    </div>
  );
}
