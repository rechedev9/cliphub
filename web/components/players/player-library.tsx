'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { PlugZap, Swords, User } from 'lucide-react';
import { isLibraryOffline, listLibraryPlayers, type LibraryPlayer } from '@/lib/api/library';
import { MATCHES_HREF, playerHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { useTableView } from '@/components/records/use-table-view';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { Button } from '@/components/ui/button';

/** The team the player appeared with most recently, by the last date of each team. */
function latestTeam(player: LibraryPlayer): string {
  const teams = [...(player.teams ?? [])].sort((a, b) => Date.parse(b.last_played_at) - Date.parse(a.last_played_at));
  return teams[0]?.team_name ?? '';
}

function lastSeen(player: LibraryPlayer): number {
  const seen = (player.nicks ?? []).map((nick) => Date.parse(nick.last_seen_at)).filter((at) => !Number.isNaN(at));
  return seen.length > 0 ? Math.max(...seen) : Date.parse(player.updated_at) || 0;
}

const COLUMNS: readonly DataTableColumn<LibraryPlayer>[] = [
  {
    id: 'nick',
    header: 'Jugador',
    required: true,
    cell: (player) => (
      <span className="inline-flex items-center gap-2">
        <PlayerAvatar nickname={player.nick || player.steamid64} playerID={player.steamid64} size={22} />
        {player.nick || player.steamid64}
      </span>
    ),
    sortValue: (player) => player.nick || player.steamid64,
  },
  { id: 'team', header: 'Equipo', cell: (player) => <span className="text-fg-2">{latestTeam(player)}</span>, sortValue: (player) => latestTeam(player) },
  { id: 'matches', header: 'Partidas', align: 'end', cell: (player) => player.match_count, sortValue: (player) => player.match_count },
  {
    id: 'seen',
    header: 'Última vez',
    cell: (player) => <span className="text-fg-2">{formatShortDate(lastSeen(player))}</span>,
    sortValue: (player) => lastSeen(player),
  },
];

/** Jugadores: every player the library has seen in its demos, the way in to a player's statistics. */
export function PlayerLibrary(): ReactNode {
  const [players, setPlayers] = useState<LibraryPlayer[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const view = useTableView('jugadores', {
    sort: { column: 'matches', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });

  useEffect(() => {
    let active = true;
    listLibraryPlayers()
      .then((list) => {
        if (!active) return;
        setPlayers(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setPlayers([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  if (players === null) return <TableSkeleton label="Cargando jugadores" columns={4} />;

  if (players.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer los jugadores. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={User}
        title="Aún no hay jugadores"
        description="Los jugadores salen de las demos que entran en Partidas: cada SteamID64 tiene su ficha con sus nicks, sus equipos y sus cifras."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={MATCHES_HREF}>
              <Swords aria-hidden />
              Importar demos
            </Link>
          </Button>
        }
      />
    );
  }

  const shown = filterRows(players, view.state.filters, {
    fields: {},
    searchText: (player) => [player.nick, player.steamid64, ...(player.nicks ?? []).map((nick) => nick.nick)].join(' '),
  });
  return (
    <DataTable
      toolbar={
        <FilterBar
          values={view.state.filters}
          onChange={view.setFilter}
          onReset={view.resetFilters}
          searchPlaceholder="Buscar por nick o SteamID64"
          count={{ shown: shown.length, total: players.length, noun: players.length === 1 ? 'jugador' : 'jugadores' }}
        />
      }
      label="Jugadores"
      rows={shown}
      columns={COLUMNS}
      rowKey={(player) => player.steamid64}
      rowHref={(player) => playerHref(player.steamid64)}
      view={view}
      empty={<p className="text-body-sm text-fg-2">Ningún jugador cumple esta búsqueda.</p>}
    />
  );
}
