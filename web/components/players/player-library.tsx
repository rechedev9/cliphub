'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { PlugZap, Swords, Users } from 'lucide-react';
import { isLibraryOffline, listLibraryPlayers, type LibraryPlayer } from '@/lib/api/library';
import { MATCHES_HREF, playerHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

const FACEIT_OPTIONS = [
  { value: 'linked', label: 'Con FACEIT' },
  { value: 'unlinked', label: 'Sin FACEIT' },
];

const COLUMNS: readonly DataTableColumn<LibraryPlayer>[] = [
  { id: 'nick', header: 'Jugador', required: true, cell: (player) => player.nick || player.steamid64, sortValue: (player) => player.nick },
  { id: 'matches', header: 'Partidas', align: 'end', cell: (player) => player.match_count, sortValue: (player) => player.match_count },
  {
    id: 'faceit',
    header: 'FACEIT',
    cell: (player) => <span className="text-fg-2">{player.faceit_nickname ?? ''}</span>,
    sortValue: (player) => player.faceit_nickname ?? null,
  },
  { id: 'steamid', header: 'SteamID64', cell: (player) => <span className="font-mono text-meta text-fg-3">{player.steamid64}</span> },
  {
    id: 'updated',
    header: 'Actualizado',
    cell: (player) => <span className="text-fg-2">{formatShortDate(player.updated_at)}</span>,
    sortValue: (player) => Date.parse(player.updated_at) || null,
  },
];

/** Jugadores: every player of the library's demos, keyed by SteamID64. */
export function PlayerLibrary(): ReactNode {
  const [players, setPlayers] = useState<LibraryPlayer[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const view = useTableView('jugadores', {
    sort: { column: 'matches', direction: 'desc' },
    hidden: ['steamid'],
    filters: { [SEARCH_FILTER]: FILTER_ANY, faceit: FILTER_ANY },
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

  if (players === null) {
    return (
      <div role="status" aria-label="Cargando jugadores" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

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
        icon={Users}
        title="Aún no hay jugadores"
        description="Cada jugador de las demos de Partidas tiene aquí su ficha, con sus nicks, sus equipos, sus partidas y el rol que se lee de sus rondas."
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
    fields: { faceit: (player) => (player.faceit_player_id ? 'linked' : 'unlinked') },
    searchText: (player) => [player.nick, player.steamid64, player.faceit_nickname ?? ''].join(' '),
  });

  return (
    <div className="flex flex-col gap-3">
      <FilterBar
        values={view.state.filters}
        onChange={view.setFilter}
        onReset={view.resetFilters}
        searchPlaceholder="Buscar por nick, SteamID64 o FACEIT"
        filters={[{ id: 'faceit', label: 'FACEIT', options: FACEIT_OPTIONS }]}
        count={{ shown: shown.length, total: players.length, noun: players.length === 1 ? 'jugador' : 'jugadores' }}
      />
      <DataTable
        label="Jugadores"
        rows={shown}
        columns={COLUMNS}
        rowKey={(player) => player.steamid64}
        rowHref={(player) => playerHref(player.steamid64)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ningún jugador cumple esta búsqueda.</p>}
      />
    </div>
  );
}
