import { Suspense, type ReactNode } from 'react';
import { PartidaRecord } from '@/components/matches/partida-record';
import { TacticalWorkspaceSkeleton } from '@/components/tactical/tactical-workspace-skeleton';

/**
 * One partida's record. The route stays a server component so the client
 * boundary sits on the record, and the Suspense boundary lets the tabs and the
 * analysis read the tab, round and filter out of the query string.
 */
export default async function MatchPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return (
    <Suspense fallback={<TacticalWorkspaceSkeleton />}>
      <PartidaRecord id={id} />
    </Suspense>
  );
}
