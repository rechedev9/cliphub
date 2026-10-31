import type { ReactNode } from 'react';
import { PlayRecord } from '@/components/stratbook/play-record';

/** One saved stratbook play. */
export default async function PlayPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <PlayRecord playId={id} />
    </div>
  );
}
