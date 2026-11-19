'use client';

import type { ReactNode } from 'react';
import { Trophy } from 'lucide-react';
import { PLAYER_TABS, type PlayerTab } from '@/lib/followed-players';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';

/** The left rail of Jugadores FACEIT: the user's follows and the ranking, one tab each. */
export function PlayerRail({ tab, onTabChange, followedCount, seguidos, ranking }: {
  tab: PlayerTab;
  onTabChange: (tab: PlayerTab) => void;
  followedCount: number;
  seguidos: ReactNode;
  ranking: ReactNode;
}): ReactNode {
  return (
    // Sticky beside the long history on wide layouts, so switching player never needs a scroll back up.
    <section className="studio-panel min-w-0 self-start overflow-hidden @[64rem]/content:sticky @[64rem]/content:top-[calc(var(--shell-strip-height)+1.5rem)]"
      aria-labelledby="player-rail-heading">
      <Tabs value={tab} onValueChange={(value) => {
        const next = PLAYER_TABS.find((entry) => entry.id === value);
        if (next !== undefined) onTabChange(next.id);
      }} className="gap-0">
        <div className="p-4">
          <h2 id="player-rail-heading" className="sr-only">Listas de jugadores</h2>
          <TabsList aria-label="Listas de jugadores" className="w-full">
            <TabsTrigger value="seguidos">
              Seguidos <span className="tabular-nums text-fg-3">{followedCount}</span>
            </TabsTrigger>
            <TabsTrigger value="ranking"><Trophy aria-hidden /> Ranking</TabsTrigger>
          </TabsList>
        </div>
        <TabsContent value="seguidos">{seguidos}</TabsContent>
        <TabsContent value="ranking">{ranking}</TabsContent>
      </Tabs>
    </section>
  );
}
