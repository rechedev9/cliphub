'use client';

import type { ReactNode } from 'react';
import { Check, Users } from 'lucide-react';
import type { LibraryRosterEntry } from '@/lib/api/library';
import type { TeamLineups } from '@/lib/api/team-insights';
import { checkedPlayers, lineupLabel, lineupMode, toggleLineupPlayer } from '@/lib/analysis/lineup';
import type { ScopeLineup } from '@/lib/analysis/scope';
import { prettyMapName, timeAgo } from '@/lib/format';
import { TEAM_RED_BUTTON } from '@/components/analysis/scope-field-style';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { Button } from '@/components/ui/button';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

export type LineupPickerProps = {
  lineup: ScopeLineup;
  lineups: TeamLineups;
  onChange: (lineup: ScopeLineup) => void;
};

const SECTION = 'px-2 pt-2 pb-1 text-meta font-medium tracking-wider text-fg-3 uppercase';

/**
 * Which of the team's players the view counts, as the button that sits by the team: its number is how
 * many are checked. The popover offers the latest five, everyone, or any set checked by hand.
 */
export function LineupPicker({ lineup, lineups, onChange }: LineupPickerProps): ReactNode {
  const checked = checkedPlayers(lineup, lineups);
  const latest = lineups.latest_match;
  const mode = lineupMode(lineup, lineups);
  const label = lineupLabel(lineup, lineups);
  const count = [...lineups.latest, ...lineups.others].filter((player) => checked.has(player.steamid64)).length;

  function row(player: LibraryRosterEntry, note: string): ReactNode {
    const on = checked.has(player.steamid64);
    const name = player.nick || player.steamid64;
    return (
      <li key={player.steamid64}>
        <label className="flex min-h-8 cursor-pointer items-center gap-2 rounded-sm px-2 text-body-sm text-popover-foreground hover:bg-surface-5 has-disabled:cursor-not-allowed">
          <input
            type="checkbox"
            checked={on}
            disabled={on && checked.size === 1}
            onChange={() => onChange(toggleLineupPlayer({ lineup, lineups, steamId: player.steamid64 }))}
            className="size-3.5 accent-primary"
          />
          <PlayerAvatar nickname={name} playerID={player.steamid64} size={20} />
          <span className="min-w-0 flex-1 truncate">{name}</span>
          <span className="text-meta whitespace-nowrap text-fg-3 tabular-nums">{note}</span>
        </label>
      </li>
    );
  }

  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`Alineación: ${mode}, ${label}`}
          title={`${mode}, ${label}`}
          className={cn('h-7 gap-1 px-2 text-body-sm tabular-nums', TEAM_RED_BUTTON, lineup.kind !== 'latest' && 'border-primary')}
        >
          <Users aria-hidden className="size-3.5" />
          {count}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="max-h-96 w-72 overflow-y-auto p-0">
        <div className="flex items-center gap-3 border-b border-border px-2 py-1.5">
          <h3 className="flex-1 text-body-sm font-semibold text-fg-1">Alineación</h3>
          <button
            type="button"
            onClick={() => onChange({ kind: 'all' })}
            className="flex items-center gap-1 text-meta text-fg-2 hover:text-fg-1"
          >
            {mode === 'Todos los jugadores' ? <Check aria-hidden className="size-3 text-primary" /> : null}
            Todos
          </button>
          {lineups.latest.length > 0 ? (
            <button
              type="button"
              onClick={() => onChange({ kind: 'latest' })}
              className="flex items-center gap-1 text-meta text-fg-2 hover:text-fg-1"
            >
              {mode === 'Última alineación' ? <Check aria-hidden className="size-3 text-primary" /> : null}
              Última
            </button>
          ) : null}
        </div>
        {lineups.latest.length > 0 ? (
          <section aria-label="Última partida">
            <h4 className={SECTION}>
              {latest === null ? 'Última partida' : `Última partida · ${prettyMapName(latest.map)} · ${timeAgo(latest.played_at)}`}
            </h4>
            <ul>{lineups.latest.map((player) => row(player, `${player.matches} ${player.matches === 1 ? 'partida' : 'partidas'}`))}</ul>
          </section>
        ) : null}
        {lineups.others.length > 0 ? (
          <section aria-label="Otros jugadores">
            <h4 className={SECTION}>Otros jugadores</h4>
            <ul>{lineups.others.map((player) => row(player, timeAgo(player.last_played_at)))}</ul>
          </section>
        ) : null}
        <p className="border-t border-border px-2 py-1.5 text-meta text-fg-3">
          Cuenta una ronda solo si todos los que jugaron en ella están marcados.
        </p>
      </PopoverContent>
    </Popover>
  );
}
