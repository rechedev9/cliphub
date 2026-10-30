'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { PlugZap, Shield, Swords } from 'lucide-react';
import { isLibraryOffline, listLibraryTeams, type LibraryTeam } from '@/lib/api/library';
import { MATCHES_HREF, teamHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

/** The aliases a team is known by besides its own name. */
function otherAliases(team: LibraryTeam): string[] {
  return team.aliases.filter((alias) => alias.toLowerCase() !== team.name.toLowerCase());
}

const COLUMNS: readonly DataTableColumn<LibraryTeam>[] = [
  { id: 'name', header: 'Equipo', required: true, cell: (team) => team.name, sortValue: (team) => team.name },
  {
    id: 'aliases',
    header: 'Alias',
    cell: (team) => <span className="text-fg-2">{otherAliases(team).join(', ')}</span>,
  },
  { id: 'matches', header: 'Partidas', align: 'end', cell: (team) => team.match_count, sortValue: (team) => team.match_count },
  {
    id: 'updated',
    header: 'Actualizado',
    cell: (team) => <span className="text-fg-2">{formatShortDate(team.updated_at)}</span>,
    sortValue: (team) => Date.parse(team.updated_at) || null,
  },
];

/** Equipos: every team the library resolved from the clan names of its demos. */
export function TeamLibrary(): ReactNode {
  const [teams, setTeams] = useState<LibraryTeam[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const view = useTableView('equipos', {
    sort: { column: 'matches', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });

  useEffect(() => {
    let active = true;
    listLibraryTeams()
      .then((list) => {
        if (!active) return;
        setTeams(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setTeams([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  if (teams === null) {
    return (
      <div role="status" aria-label="Cargando equipos" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (teams.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer los equipos. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={Shield}
        title="Aún no hay equipos"
        description="Los equipos se crean solos a partir del nombre de clan de cada demo que entra en Partidas, y se corrigen a mano desde la partida."
        preview={[
          'Plantilla por fecha de cada equipo.',
          'Sus partidas, con el rival y el resultado.',
          'Rendimiento por mapa, con las rondas ganadas y perdidas.',
        ]}
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

  const shown = filterRows(teams, view.state.filters, {
    fields: {},
    searchText: (team) => [team.name, ...team.aliases].join(' '),
  });

  return (
    <div className="flex flex-col gap-3">
      <FilterBar
        values={view.state.filters}
        onChange={view.setFilter}
        onReset={view.resetFilters}
        searchPlaceholder="Buscar por nombre o alias"
        count={{ shown: shown.length, total: teams.length, noun: teams.length === 1 ? 'equipo' : 'equipos' }}
      />
      <DataTable
        label="Equipos"
        rows={shown}
        columns={COLUMNS}
        rowKey={(team) => team.id}
        rowHref={(team) => teamHref(team.id)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ningún equipo cumple esta búsqueda.</p>}
      />
    </div>
  );
}
