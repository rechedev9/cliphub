import type { ReactNode } from 'react';
import { RankingBoard } from '@/components/ranking/ranking-board';
import { StudioPageHeader } from '@/components/studio/page-header';

/** Ranking: the library's players ordered by one metric, against the same sample as every comparison. */
export default function RankingPage(): ReactNode {
  return (
    <div className="measure-work flex flex-col gap-5">
      <StudioPageHeader title="Ranking" description="Los jugadores de tus demos ordenados por una cifra, en todos los mapas o en uno." />
      <RankingBoard />
    </div>
  );
}
