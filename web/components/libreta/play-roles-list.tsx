'use client';

import { useMemo, type ReactNode } from 'react';
import Link from 'next/link';
import { BookMarked } from 'lucide-react';
import type { LibraryPlay } from '@/lib/api/stratbook';
import { LIBRETA_QUERY, LIBRETA_TABS, libretaTabHref, playHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import type { CalloutLabel } from '@/lib/libreta/callouts';
import { playRoleRows, type PlayRoleRow } from '@/lib/libreta/roles';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { playKindLabel, playShapeLabel } from '@/lib/stratbook/plays';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { STATUS_FILTER_OPTIONS, StatusBadge } from './status-badge';

function Missing({ text }: { text: string }): ReactNode {
  return <span className="text-fg-3">{text}</span>;
}

function roleColumns(calloutLabel: CalloutLabel): DataTableColumn<PlayRoleRow>[] {
  const callout = (row: PlayRoleRow): string => calloutLabel(row.play.map, row.slot.callout ?? '');
  return [
    { id: 'play', header: 'Jugada', required: true, cell: (row) => row.play.title, sortValue: (row) => row.play.title },
    { id: 'side', header: 'Lado', cell: (row) => row.play.side, sortValue: (row) => row.play.side },
    { id: 'kind', header: 'Tipo', cell: (row) => playShapeLabel(row.play), sortValue: (row) => row.play.kind },
    { id: 'role', header: 'Rol', cell: (row) => row.slot.name, sortValue: (row) => row.slot.name },
    { id: 'player', header: 'Jugador', cell: (row) => row.player || <Missing text="Sin jugador" />, sortValue: (row) => row.player },
    {
      id: 'callout',
      header: 'Callout',
      cell: (row) => callout(row) || <Missing text="Sin callout" />,
      sortValue: callout,
    },
    { id: 'task', header: 'Tarea', cell: (row) => row.slot.task ?? '', sortValue: (row) => row.slot.task ?? '' },
    { id: 'status', header: 'Estado', cell: (row) => <StatusBadge status={row.play.status} />, sortValue: (row) => row.play.status },
  ];
}

/** The roles written in the Libreta's plays of one map and team, a row per role, each opening its play. */
export function PlayRolesList({
  plays,
  map,
  team,
  teamName,
  names,
}: {
  plays: readonly LibraryPlay[];
  map: string;
  /** A team id, or null for the plays without a team. */
  team: string | null;
  teamName?: string;
  names: ReadonlyMap<string, string>;
}): ReactNode {
  const view = useTableView('libreta-roles', {
    sort: { column: 'play', direction: 'asc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const calloutLabel = useCalloutNames([map]);
  const columns = useMemo(() => roleColumns(calloutLabel), [calloutLabel]);
  const rows = playRoleRows(plays, { map, team, names });
  const owner = team === null ? 'sin equipo' : `de ${teamName ?? 'este equipo'}`;

  if (rows.length === 0) {
    return (
      <EmptyState
        icon={BookMarked}
        title={`Ninguna jugada ${owner} en ${prettyMapName(map)} tiene roles`}
        description="Abre una jugada y pulsa Editar roles en su resumen. Studio parte de las posiciones guardadas en la jugada."
        actions={
          <Button asChild variant="outline" size="sm">
            <Link href={libretaTabHref(LIBRETA_TABS.plays, team === null ? {} : { [LIBRETA_QUERY.team]: team })}>Ver las jugadas</Link>
          </Button>
        }
      />
    );
  }

  const filtered = filterRows(rows, view.state.filters, {
    fields: {
      side: (row) => row.play.side,
      kind: (row) => row.play.kind,
      status: (row) => row.play.status,
    },
    searchText: (row) => [row.play.title, row.slot.name, row.player, calloutLabel(map, row.slot.callout ?? ''), row.slot.task ?? ''].join(' '),
  });
  const anyStatus = (view.state.filters.status ?? FILTER_ANY) === FILTER_ANY;
  const shown = anyStatus ? filtered.filter((row) => row.play.status !== 'archived') : filtered;
  const kinds = [...new Map(rows.map((row) => [row.play.kind, playKindLabel(row.play.kind)]))]
    .map(([value, label]) => ({ value, label }))
    .sort((a, b) => a.label.localeCompare(b.label, 'es'));

  return (
    <div className="flex flex-col gap-3">
      {anyStatus && filtered.length > shown.length ? (
        <p className="text-meta text-fg-3">Los roles de las jugadas archivadas no se muestran. Elige Estado: Archivo para verlos.</p>
      ) : null}
      <DataTable
        toolbar={
          <FilterBar
            values={view.state.filters}
            onChange={view.setFilter}
            onReset={view.resetFilters}
            searchPlaceholder="Buscar por jugada, rol, jugador o tarea"
            filters={[
              { id: 'side', label: 'Lado', options: [{ value: 'T', label: 'T' }, { value: 'CT', label: 'CT' }] },
              { id: 'kind', label: 'Tipo', options: kinds },
              { id: 'status', label: 'Estado', options: STATUS_FILTER_OPTIONS },
            ]}
            count={{ shown: shown.length, total: rows.length, noun: rows.length === 1 ? 'rol' : 'roles' }}
          />
        }
        label="Roles de las jugadas"
        rows={shown}
        columns={columns}
        rowKey={(row) => row.key}
        rowHref={(row) => playHref(row.play.id)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ningún rol cumple estos filtros.</p>}
      />
    </div>
  );
}
