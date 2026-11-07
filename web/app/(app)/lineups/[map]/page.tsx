import type { ReactNode } from 'react';
import { LineupExplorer } from '@/components/pro-lineups/lineup-explorer';

/** One map's pro lineups and combos. */
export default async function LineupMapPage({ params }: { params: Promise<{ map: string }> }): Promise<ReactNode> {
  const { map } = await params;
  return (
    <div className="measure-work flex flex-col gap-5">
      <LineupExplorer map={map} />
    </div>
  );
}
