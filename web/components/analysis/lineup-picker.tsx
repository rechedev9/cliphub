'use client';

import type { ReactNode } from 'react';
import { Check, Users } from 'lucide-react';
import type { LibraryRosterEntry } from '@/lib/api/library';
import type { TeamLineups } from '@/lib/api/team-insights';
import { checkedPlayers, lineupLabel, toggleLineupPlayer } from '@/lib/analysis/lineup';
import type { ScopeLineup } from '@/lib/analysis/scope';
import { prettyMapName, timeAgo } from '@/lib/format';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { cn } from '@/lib/utils';

export type LineupPickerProps = {
  lineup: ScopeLineup;
  lineups: TeamLineups;
  onChange: (lineup: ScopeLineup) => void;
};

/**
 * Which of the team's players the view counts: the latest match's five by
 * default, everyone, or any set checked by hand.
 */
export function LineupPicker({ lineup, lineups, onChange }: LineupPickerProps): ReactNode {
  const checked = checkedPlayers(lineup, lineups);
  const latest = lineups.latest_match;

  function row(player: LibraryRosterEntry, note: string): ReactNode {
    const on = checked.has(player.steamid64);
    return (
      <DropdownMenuCheckboxItem
        key={player.steamid64}
        checked={on}
        disabled={on && checked.size === 1}
        onSelect={(event) => {
          // Keep the menu open so several players can be checked in a row.
          event.preventDefault();
        }}
        onCheckedChange={() => onChange(toggleLineupPlayer(lineup, lineups, player.steamid64))}
      >
        <span className="min-w-0 flex-1 truncate">{player.nick || player.steamid64}</span>
        <span className="text-meta whitespace-nowrap text-fg-3 tabular-nums">{note}</span>
      </DropdownMenuCheckboxItem>
    );
  }

  function shortcut(target: ScopeLineup, label: string): ReactNode {
    return (
      <DropdownMenuItem onSelect={() => onChange(target)}>
        <span className="flex-1">{label}</span>
        {lineup.kind === target.kind ? <Check aria-label="Elegida" className="text-fg-1" /> : null}
      </DropdownMenuItem>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`Alineación: ${lineupLabel(lineup)}`}
          className={cn('h-8 gap-1.5 px-2.5 text-body-sm', lineup.kind !== 'latest' && 'border-fg-3 text-fg-1')}
        >
          <Users aria-hidden />
          {lineupLabel(lineup)}
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="w-72">
        {shortcut({ kind: 'latest' }, 'Última alineación')}
        {shortcut({ kind: 'all' }, 'Todos los jugadores')}
        {lineups.latest.length > 0 ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>
              {latest === null ? 'Última partida' : `Última partida · ${prettyMapName(latest.map)} · ${timeAgo(latest.played_at)}`}
            </DropdownMenuLabel>
            {lineups.latest.map((player) => row(player, `${player.matches} ${player.matches === 1 ? 'partida' : 'partidas'}`))}
          </>
        ) : null}
        {lineups.others.length > 0 ? (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuLabel>Otros jugadores</DropdownMenuLabel>
            {lineups.others.map((player) => row(player, timeAgo(player.last_played_at)))}
          </>
        ) : null}
        <DropdownMenuSeparator />
        <p className="px-2 py-1.5 text-meta text-fg-3">Cuenta una ronda solo si todos los que jugaron en ella están marcados.</p>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
