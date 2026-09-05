'use client';

import { useState, type ReactNode } from 'react';
import { Search } from 'lucide-react';
import type { FaceitFollowedPlayer } from '@/lib/api/faceit';
import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import { FOCUS_RING } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { PlayerAvatar } from './player-avatar';
import { LevelBadge } from './level-badge';

const SORT = { elo: 'elo', name: 'name' } as const;

export function FollowedPlayerList({ players, selectedID, onSelect }: {
  players: FaceitFollowedPlayer[];
  selectedID: string | null;
  onSelect: (id: string) => void;
}): ReactNode {
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<string>(SORT.elo);
  const filtered = players.filter((player) => player.nickname.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
  filtered.sort((a, b) => {
    if (sort === SORT.name) return a.nickname.localeCompare(b.nickname);
    return (b.elo ?? -1) - (a.elo ?? -1) || a.nickname.localeCompare(b.nickname);
  });

  return (
    <section className="studio-panel min-w-0 self-start overflow-hidden" aria-labelledby="followed-players-heading">
      <div className="space-y-4 p-4">
        <div className="flex items-center gap-2.5">
          <h2 id="followed-players-heading" className="text-body font-semibold text-fg-1">Siguiendo</h2>
          <span className="rounded-full border border-border bg-surface-3 px-2 text-body-sm tabular-nums text-fg-2">{players.length}</span>
        </div>
        <div className="flex gap-2">
          <div className="relative min-w-0 flex-1">
            <Search aria-hidden className="pointer-events-none absolute top-3.5 left-3 size-4 text-fg-3" />
            <Input aria-label="Buscar jugador seguido" placeholder="Buscar jugador" value={query}
              onChange={(event) => setQuery(event.target.value)} className="pl-9" />
          </div>
          <Select value={sort} onValueChange={setSort}>
            <SelectTrigger aria-label="Ordenar jugadores" className="w-24 shrink-0"><SelectValue /></SelectTrigger>
            <SelectContent><SelectItem value={SORT.elo}>ELO ↓</SelectItem><SelectItem value={SORT.name}>A–Z</SelectItem></SelectContent>
          </Select>
        </div>
      </div>
      <nav aria-label="Jugadores seguidos" className="max-h-72 overflow-y-auto overscroll-contain border-t border-border @[64rem]/content:max-h-[calc(100dvh-18rem)]">
        {filtered.map((player) => (
          <button key={player.id} type="button" onClick={() => onSelect(player.id)}
            aria-current={selectedID === player.id ? 'true' : undefined}
            className={cn('flex min-h-16 w-full items-center gap-3 border-b border-border-subtle px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-surface-3 focus-visible:-outline-offset-2',
              FOCUS_RING, selectedID === player.id && 'bg-accent shadow-[inset_3px_0_0_var(--primary)]')}>
            <PlayerAvatar nickname={player.nickname} playerID={player.id} size={36} />
            <span className="min-w-0 flex-1 truncate text-body-sm font-semibold text-fg-1" title={player.nickname}>{player.nickname}</span>
            <LevelBadge level={player.skill_level} />
            <span className="w-16 shrink-0 text-right text-body-sm tabular-nums text-fg-1">
              {player.elo ?? '—'} <span className="text-meta tracking-normal text-fg-3">ELO</span>
            </span>
          </button>
        ))}
        {filtered.length === 0 ? <p role="status" className="px-4 py-8 text-center text-body-sm text-fg-2">No hay jugadores que coincidan con tu búsqueda.</p> : null}
      </nav>
    </section>
  );
}
