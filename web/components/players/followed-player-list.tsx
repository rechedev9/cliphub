'use client';

import { useState, type ReactNode } from 'react';
import { Search } from 'lucide-react';
import type { FaceitFollowedPlayer } from '@/lib/api/faceit';
import { inPlayerTab, PLAYER_TABS, type PlayerTab } from '@/lib/followed-players';
import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import { FOCUS_RING } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { PlayerAvatar } from './player-avatar';
import { LevelBadge } from './level-badge';

const SORT = { elo: 'elo', name: 'name' } as const;

export function FollowedPlayerList({ players, tab, onTabChange, selectedID, onSelect }: {
  players: FaceitFollowedPlayer[];
  tab: PlayerTab;
  onTabChange: (tab: PlayerTab) => void;
  selectedID: string | null;
  onSelect: (id: string) => void;
}): ReactNode {
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<string>(SORT.elo);
  const needle = query.trim().toLocaleLowerCase();
  const inTab = players.filter((player) => inPlayerTab(player, tab));
  const filtered = inTab.filter((player) => player.nickname.toLocaleLowerCase().includes(needle));
  filtered.sort((a, b) => {
    if (sort === SORT.name) return a.nickname.localeCompare(b.nickname);
    return (b.elo ?? -1) - (a.elo ?? -1) || a.nickname.localeCompare(b.nickname);
  });
  const label = PLAYER_TABS.find((entry) => entry.id === tab)?.label ?? tab;
  let empty = 'No hay jugadores que coincidan con tu búsqueda.';
  if (inTab.length === 0) {
    empty = tab === 'custom' ? 'Aún no sigues a nadie. Busca un nick o pega una URL de FACEIT arriba.' : 'No quedan jugadores en esta lista.';
  }

  return (
    // Sticky beside the long history on wide layouts, so switching player never needs a scroll back up.
    <section className="studio-panel min-w-0 self-start overflow-hidden @[64rem]/content:sticky @[64rem]/content:top-[calc(var(--shell-strip-height)+1.5rem)]"
      aria-labelledby="followed-players-heading">
      <Tabs value={tab} onValueChange={(value) => onTabChange(value as PlayerTab)} className="gap-0">
        <div className="space-y-4 p-4">
          <h2 id="followed-players-heading" className="sr-only">Listas de jugadores</h2>
          <TabsList aria-label="Listas de jugadores" className="w-full">
            {PLAYER_TABS.map((entry) => (
              <TabsTrigger key={entry.id} value={entry.id}>
                {entry.label}
                <span className="tabular-nums text-fg-3">{players.filter((player) => inPlayerTab(player, entry.id)).length}</span>
              </TabsTrigger>
            ))}
          </TabsList>
          <div className="flex gap-2">
            <div className="relative min-w-0 flex-1">
              <Search aria-hidden className="pointer-events-none absolute top-3.5 left-3 size-4 text-fg-3" />
              <Input aria-label={`Buscar jugador en ${label}`} placeholder="Buscar jugador" value={query}
                onChange={(event) => setQuery(event.target.value)} className="pl-9" />
            </div>
            <Select value={sort} onValueChange={setSort}>
              <SelectTrigger aria-label="Ordenar jugadores" className="w-24 shrink-0"><SelectValue /></SelectTrigger>
              <SelectContent><SelectItem value={SORT.elo}>ELO ↓</SelectItem><SelectItem value={SORT.name}>A–Z</SelectItem></SelectContent>
            </Select>
          </div>
        </div>
        <TabsContent value={tab}>
          {/* Sticky, the rail must also fit a short window: past 9 rows or the viewport, whichever is lower, the list scrolls. */}
          <nav aria-label={`Jugadores ${label}`} className="max-h-[calc(4*4rem+1px)] overflow-y-auto overscroll-contain border-t border-border @[64rem]/content:max-h-[min(calc(9*4rem+1px),calc(100dvh_-_var(--shell-strip-height)_-_11rem))]">
            {filtered.map((player) => (
              <button key={player.id} type="button" onClick={() => onSelect(player.id)}
                aria-current={selectedID === player.id ? 'true' : undefined}
                className={cn('flex min-h-16 w-full items-center gap-3 border-b border-border-subtle px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-surface-3 focus-visible:-outline-offset-2',
                  FOCUS_RING, selectedID === player.id && 'bg-accent shadow-[inset_3px_0_0_var(--primary)]')}>
                <PlayerAvatar nickname={player.nickname} playerID={player.id} avatar={player.avatar} size={36} />
                <span className="min-w-0 flex-1 truncate text-body-sm font-semibold text-fg-1" title={player.nickname}>{player.nickname}</span>
                <LevelBadge level={player.skill_level} />
                <span className="w-16 shrink-0 text-right text-body-sm tabular-nums text-fg-1">
                  {player.elo ?? '—'} <span className="text-meta tracking-normal text-fg-3">ELO</span>
                </span>
              </button>
            ))}
            {filtered.length === 0 ? <p role="status" className="px-4 py-8 text-center text-body-sm text-fg-2">{empty}</p> : null}
          </nav>
        </TabsContent>
      </Tabs>
    </section>
  );
}
