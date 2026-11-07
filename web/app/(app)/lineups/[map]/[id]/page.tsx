import type { ReactNode } from 'react';
import { LineupDetail } from '@/components/pro-lineups/lineup-detail';

/** One pro lineup. */
export default async function ProLineupPage({ params }: { params: Promise<{ map: string; id: string }> }): Promise<ReactNode> {
  const { map, id } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <LineupDetail map={map} id={decodeURIComponent(id)} />
    </div>
  );
}
