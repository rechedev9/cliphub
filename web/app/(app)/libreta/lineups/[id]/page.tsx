import type { ReactNode } from 'react';
import { LineupRecord } from '@/components/libreta/lineup-record';

/** One saved lineup of the Libreta. */
export default async function LineupPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <LineupRecord lineupId={id} />
    </div>
  );
}
