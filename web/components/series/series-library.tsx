'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { Layers, Plus, PlugZap } from 'lucide-react';
import { isLibraryOffline, listLibrarySeries } from '@/lib/api/library';
import type { LibrarySeries } from '@/lib/api/veto';
import { librarySeriesHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { SERIES_FORMAT_LABEL } from '@/lib/library/veto';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { NewSeriesDialog } from './new-series-dialog';

/** `A vs B`, or the event, or the format when the series names nothing. */
export function seriesName(series: LibrarySeries): string {
  if (series.teams.length === 2) return `${series.teams[0]?.name} vs ${series.teams[1]?.name}`;
  return series.event || `Serie ${SERIES_FORMAT_LABEL[series.format]}`;
}

export const SERIES_COLUMNS: readonly DataTableColumn<LibrarySeries>[] = [
  { id: 'name', header: 'Serie', required: true, cell: (s) => seriesName(s), sortValue: (s) => seriesName(s) },
  { id: 'event', header: 'Evento', cell: (s) => <span className="text-fg-2">{s.event}</span>, sortValue: (s) => s.event },
  { id: 'format', header: 'Formato', cell: (s) => SERIES_FORMAT_LABEL[s.format], sortValue: (s) => s.format },
  { id: 'maps', header: 'Mapas', align: 'end', cell: (s) => s.match_ids.length, sortValue: (s) => s.match_ids.length },
  { id: 'veto', header: 'Pasos de veto', align: 'end', cell: (s) => s.vetoes.length, sortValue: (s) => s.vetoes.length },
  {
    id: 'created',
    header: 'Creada',
    cell: (s) => <span className="text-fg-2">{formatShortDate(s.created_at)}</span>,
    sortValue: (s) => Date.parse(s.created_at) || null,
  },
];

/** Every library series, with the button that starts a new one. */
export function SeriesLibrary(): ReactNode {
  const [series, setSeries] = useState<LibrarySeries[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [creating, setCreating] = useState(false);
  const view = useTableView('series', { sort: { column: 'created', direction: 'desc' }, hidden: [], filters: { [SEARCH_FILTER]: FILTER_ANY } });

  useEffect(() => {
    let active = true;
    listLibrarySeries()
      .then((list) => {
        if (!active) return;
        setSeries(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setSeries([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  const newButton = (
    <Button type="button" size="sm" onClick={() => setCreating(true)}>
      <Plus aria-hidden />
      Nueva serie
    </Button>
  );

  let body: ReactNode;
  if (series === null) {
    body = (
      <div role="status" aria-label="Cargando series" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  } else if (series.length === 0) {
    body = offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer las series. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={Layers}
        title="Aún no hay series"
        description="Una serie junta los mapas de un BO1, BO3 o BO5 y guarda su veto. Con varias series apuntadas, el simulador de veto lee los picks y bans de cada rival."
        preview={['Los dos equipos y el formato.', 'Sus mapas, con el resultado de cada uno.', 'El orden de picks y bans.']}
        actions={newButton}
      />
    );
  } else {
    const shown = filterRows(series, view.state.filters, {
      fields: {},
      searchText: (s) => [seriesName(s), s.event].join(' '),
    });
    body = (
      <div className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <FilterBar
            className="flex flex-1 flex-wrap items-center gap-2"
            values={view.state.filters}
            onChange={view.setFilter}
            onReset={view.resetFilters}
            searchPlaceholder="Buscar por equipo o evento"
            count={{ shown: shown.length, total: series.length, noun: series.length === 1 ? 'serie' : 'series' }}
          />
          {newButton}
        </div>
        <DataTable
          label="Series"
          rows={shown}
          columns={SERIES_COLUMNS}
          rowKey={(s) => s.id}
          rowHref={(s) => librarySeriesHref(s.id)}
          view={view}
          empty={<p className="text-body-sm text-fg-2">Ninguna serie cumple esta búsqueda.</p>}
        />
      </div>
    );
  }

  return (
    <>
      {body}
      {creating ? <NewSeriesDialog onClose={() => setCreating(false)} /> : null}
    </>
  );
}
