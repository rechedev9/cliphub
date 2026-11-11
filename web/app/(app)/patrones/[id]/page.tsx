import type { ReactNode } from 'react';
import { PatternsWorkspace } from '@/components/patterns/canvas/patterns-workspace';

/** The Buscador de patrones of one team: grenades, players and patterns on the map. */
export default async function TeamPatternsPage({ params }: { params: Promise<{ id: string }> }): Promise<ReactNode> {
  const { id } = await params;
  return <PatternsWorkspace teamId={id} />;
}
