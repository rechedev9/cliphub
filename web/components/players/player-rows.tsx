'use client';

import { useState, type ReactNode } from 'react';
import { Search, UserCheck } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Input } from '@/components/ui/input';
import { FOCUS_RING } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { CountryFlag } from './country-flag';
import { LevelBadge } from './level-badge';
import { PlayerAvatar } from './player-avatar';

const SORT = { elo: 'elo', name: 'name' } as const;

/** A rail row: a ranking row carries its place, a followed one its avatar. */
export type RailPlayer = {
  id: string;
  nickname: string;
  avatar?: string;
  country?: string;
  skill_level?: number;
  elo?: number;
  rank?: number;
};

/** The players of one rail list, filtered by nick and sorted by ELO or name inside what is loaded. */
export function PlayerRows({ players, label, selectedID, onSelect, empty, followedIDs, footer }: {
  players: readonly RailPlayer[];
  /** Names the list for the search field and the navigation landmark. */
  label: string;
  selectedID: string | null;
  onSelect: (id: string) => void;
  /** Shown when the list itself is empty, not when the search hides every row. */
  empty: string;
  /** Marks the rows of players the user already follows. */
  followedIDs?: ReadonlySet<string>;
  footer?: ReactNode;
}): ReactNode {
  const [query, setQuery] = useState('');
  const [sort, setSort] = useState<string>(SORT.elo);
  const needle = query.trim().toLocaleLowerCase();
  const filtered = players.filter((player) => player.nickname.toLocaleLowerCase().includes(needle));
  filtered.sort((a, b) => {
    if (sort === SORT.name) return a.nickname.localeCompare(b.nickname);
    return (b.elo ?? -1) - (a.elo ?? -1) || (a.rank ?? 0) - (b.rank ?? 0) || a.nickname.localeCompare(b.nickname);
  });

  return (
    <>
      <div className="flex gap-2 px-4 pb-4">
        <div className="relative min-w-0 flex-1">
          <Search aria-hidden className="pointer-events-none absolute top-3.5 left-3 size-4 text-fg-3" />
          <Input aria-label={`Buscar jugador en ${label}`} placeholder="Buscar jugador" value={query}
            onChange={(event) => setQuery(event.target.value)} className="pl-9" />
        </div>
        <Select value={sort} onValueChange={setSort}>
          <SelectTrigger aria-label="Ordenar jugadores" className="w-24 shrink-0"><SelectValue /></SelectTrigger>
          <SelectContent><SelectItem value={SORT.elo}>ELO ↓</SelectItem><SelectItem value={SORT.name}>A-Z</SelectItem></SelectContent>
        </Select>
      </div>
      {/* Sticky, the rail must also fit a short window: past 9 rows or the viewport, whichever is lower, the list scrolls. */}
      <nav aria-label={`Jugadores de ${label}`} className="max-h-[calc(4*4rem+1px)] overflow-y-auto overscroll-contain border-t border-border @[64rem]/content:max-h-[min(calc(9*4rem+1px),calc(100dvh_-_var(--shell-strip-height)_-_15rem))]">
        {filtered.map((player) => (
          <button key={player.id} type="button" onClick={() => onSelect(player.id)}
            aria-current={selectedID === player.id ? 'true' : undefined}
            className={cn('flex min-h-16 w-full items-center gap-3 border-b border-border-subtle px-4 py-3 text-left transition-colors last:border-b-0 hover:bg-surface-3 focus-visible:-outline-offset-2',
              FOCUS_RING, selectedID === player.id && 'bg-accent shadow-[inset_3px_0_0_var(--primary)]')}>
            {player.rank === undefined
              ? <PlayerAvatar nickname={player.nickname} playerID={player.id} avatar={player.avatar} size={36} />
              : <span className="w-7 shrink-0 text-right text-body-sm font-semibold tabular-nums text-fg-2"><span className="sr-only">Puesto </span>{player.rank}</span>}
            <span className="flex min-w-0 flex-1 items-center gap-2">
              <CountryFlag code={player.country} />
              <span className="truncate text-body-sm font-semibold text-fg-1" title={player.nickname}>{player.nickname}</span>
              {followedIDs?.has(player.id) ? <><UserCheck aria-hidden className="size-3.5 shrink-0 text-primary" /><span className="sr-only">, seguido</span></> : null}
            </span>
            <LevelBadge level={player.skill_level} />
            <span className="w-16 shrink-0 text-right text-body-sm tabular-nums text-fg-1">
              {player.elo ?? '-'} <span className="text-meta tracking-normal text-fg-3">ELO</span>
            </span>
          </button>
        ))}
        {filtered.length === 0 ? <p role="status" className="px-4 py-8 text-center text-body-sm text-fg-2">
          {players.length === 0 ? empty : 'No hay jugadores que coincidan con tu búsqueda.'}
        </p> : null}
      </nav>
      {footer}
    </>
  );
}
