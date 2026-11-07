'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useSearchParams } from 'next/navigation';
import { MapPinned, Pencil, PlugZap, Swords } from 'lucide-react';
import { getMapCallouts, type MapCallout, type MapCallouts } from '@/lib/api/callouts';
import { isLibraryOffline } from '@/lib/api/library';
import type { LibraryLineup } from '@/lib/api/lineups';
import type { LibraryPlay } from '@/lib/api/stratbook';
import { RADAR_LEVELS } from '@/lib/api/tactical';
import { LIBRETA_QUERY, MATCHES_HREF } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { CALLOUT_NAMES } from '@/lib/libreta/callouts';
import { busiestMap } from '@/lib/libreta/plan-table';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { placeLabel } from '@/lib/tactical-labels';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap } from '@/lib/tactical-transform';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyCell } from '@/components/records/empty-cell';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { CalloutMap } from './callout-map';
import { MapField, mapChoices } from './play-fields';
import { RenameCalloutDialog } from './rename-callout-dialog';

const MAP_PATTERN = /^[a-z0-9_]+$/;

const NAMED_OPTIONS = [
  { value: 'team', label: 'Con nombre del equipo' },
  { value: 'cs2', label: 'Con el nombre de CS2' },
];

/** The Callouts tab: the team's own name for each CS2 place of a map, on the map and in a list. */
export function CalloutSheet({ plays, lineups }: { plays: LibraryPlay[] | null; lineups: LibraryLineup[] | null }): ReactNode {
  const asked = useSearchParams().get(LIBRETA_QUERY.map);
  const [chosen, setChosen] = useState<string | null>(null);
  const map = chosen ?? (asked !== null && MAP_PATTERN.test(asked) ? asked : busiestMap(plays ?? [], 'de_mirage'));
  const known = [...(plays ?? []).map((play) => play.map), ...(lineups ?? []).map((lineup) => lineup.map)];
  return (
    <div className="flex flex-col gap-4">
      <div className="max-w-56">
        <MapField value={map} onChange={setChosen} maps={mapChoices([map, ...known])} />
      </div>
      <MapCalloutsView key={map} map={map} />
    </div>
  );
}

type Load = { kind: 'loading' } | { kind: 'ready'; data: MapCallouts } | { kind: 'failed'; offline: boolean };

function MapCalloutsView({ map }: { map: string }): ReactNode {
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [renaming, setRenaming] = useState<MapCallout | null>(null);
  const calloutLabel = useCalloutNames([map]);

  useEffect(() => {
    let active = true;
    getMapCallouts(map, { geometry: true })
      .then((data) => {
        if (!active) return;
        CALLOUT_NAMES.replace(map, data.callouts);
        setLoad({ kind: 'ready', data });
      })
      .catch((error: unknown) => {
        if (active) setLoad({ kind: 'failed', offline: isLibraryOffline(error) });
      });
    return () => {
      active = false;
    };
  }, [map, attempt]);

  if (load.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando callouts" className="grid gap-3 xl:grid-cols-[minmax(0,1.2fr)_minmax(24rem,1fr)]">
        <Skeleton className="aspect-square w-full max-w-3xl" />
        <Skeleton className="h-96 w-full" />
      </div>
    );
  }
  if (load.kind === 'failed') {
    return (
      <EmptyState
        icon={PlugZap}
        title={load.offline ? 'Servicio local sin conexión' : 'No se pudieron leer los callouts'}
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => {
              setLoad({ kind: 'loading' });
              setAttempt((n) => n + 1);
            }}
          >
            Reintentar
          </Button>
        }
      />
    );
  }

  const { data } = load;
  const label = (token: string): string => calloutLabel(map, token);
  function saved(token: string, name: string | null): void {
    CALLOUT_NAMES.rename({ map, token, name });
    setLoad({
      kind: 'ready',
      data: {
        ...data,
        callouts: data.callouts.map((callout) => {
          if (callout.token !== token) return callout;
          const { name: _old, ...rest } = callout;
          return name === null ? rest : { ...rest, name };
        }),
      },
    });
  }
  const multi = data.geometry !== undefined && isMultiLevelMap(data.geometry.calibration);
  const list = <CalloutList callouts={data.callouts} multiLevel={multi} onPick={setRenaming} />;
  const dialog = renaming ? (
    <RenameCalloutDialog
      map={map}
      callout={renaming}
      onClose={() => setRenaming(null)}
      onSaved={(name) => saved(renaming.token, name)}
    />
  ) : null;

  if (data.geometry === undefined) {
    return (
      <div className="flex flex-col gap-3">
        <EmptyState
          icon={MapPinned}
          title={`Analiza una partida de ${prettyMapName(map)} para ver sus callouts en el mapa`}
          description={
            data.callouts.length === 0
              ? 'Studio saca los callouts de CS2 de las partidas analizadas. Cuando haya una, podrás ponerle a cada zona el nombre que usa tu equipo.'
              : 'Studio saca los callouts de CS2 de las partidas analizadas. Mientras tanto, aquí abajo están las zonas que tu equipo ya nombró.'
          }
          actions={
            <Button asChild variant="outline" size="sm">
              <Link href={MATCHES_HREF}>
                <Swords aria-hidden />
                Ver partidas
              </Link>
            </Button>
          }
        />
        {data.callouts.length > 0 ? list : null}
        {dialog}
      </div>
    );
  }

  return (
    <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(24rem,1fr)]">
      <section aria-label="Mapa" className="w-full max-w-3xl min-w-0 xl:max-w-none">
        <CalloutMap map={map} geometry={data.geometry} callouts={data.callouts} label={label} onPick={setRenaming} />
      </section>
      <section aria-label="Lista de callouts" className="min-w-0">
        {list}
      </section>
      {dialog}
    </div>
  );
}

function calloutColumns({ multiLevel, onPick }: { multiLevel: boolean; onPick: (callout: MapCallout) => void }): DataTableColumn<MapCallout>[] {
  const columns: DataTableColumn<MapCallout>[] = [
    {
      id: 'cs2',
      header: 'Nombre de CS2',
      required: true,
      cell: (callout) => (
        <button
          type="button"
          onClick={() => onPick(callout)}
          className="rounded-sm text-left font-medium text-fg-1 underline-offset-4 outline-none hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {placeLabel(callout.token)}
        </button>
      ),
      sortValue: (callout) => placeLabel(callout.token),
    },
    {
      id: 'team',
      header: 'Nombre del equipo',
      cell: (callout) => callout.name ?? <EmptyCell label="Sin nombre propio" />,
      sortValue: (callout) => callout.name ?? '',
    },
  ];
  if (multiLevel) {
    columns.push({
      id: 'level',
      header: 'Piso',
      cell: (callout) => (callout.center === undefined ? <span className="text-fg-3">No sale en el mapa</span> : LEVEL_LABEL[callout.level ?? RADAR_LEVELS.default]),
      sortValue: (callout) => callout.level ?? RADAR_LEVELS.default,
    });
  }
  columns.push({
    id: 'edit',
    header: '',
    required: true,
    align: 'end',
    cell: (callout) => (
      <Button type="button" variant="ghost" size="xs" aria-label={`Renombrar ${placeLabel(callout.token)}`} onClick={() => onPick(callout)}>
        <Pencil aria-hidden />
        Renombrar
      </Button>
    ),
  });
  return columns;
}

function CalloutList({
  callouts,
  multiLevel,
  onPick,
}: {
  callouts: readonly MapCallout[];
  multiLevel: boolean;
  onPick: (callout: MapCallout) => void;
}): ReactNode {
  const view = useTableView('libreta-callouts', {
    sort: { column: 'cs2', direction: 'asc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const columns = useMemo(() => calloutColumns({ multiLevel, onPick }), [multiLevel, onPick]);
  const shown = filterRows(callouts, view.state.filters, {
    fields: { named: (callout) => (callout.name ? 'team' : 'cs2') },
    searchText: (callout) => `${placeLabel(callout.token)} ${callout.token} ${callout.name ?? ''}`,
  });
  const named = callouts.filter((callout) => callout.name).length;

  return (
    <div className="flex flex-col gap-3">
      <FilterBar
        values={view.state.filters}
        onChange={view.setFilter}
        onReset={view.resetFilters}
        searchPlaceholder="Buscar por nombre de CS2 o del equipo"
        filters={[{ id: 'named', label: 'Nombre', options: NAMED_OPTIONS }]}
        count={{ shown: shown.length, total: callouts.length, noun: callouts.length === 1 ? 'callout' : 'callouts' }}
        className="flex flex-wrap items-center gap-2"
      />
      {named < callouts.length ? (
        <p className="text-meta text-fg-3">
          {named === 0 ? 'Tu equipo aún no ha nombrado ninguna zona de este mapa.' : `Tu equipo ha nombrado ${named} de ${callouts.length}.`}
        </p>
      ) : null}
      <DataTable
        label="Callouts"
        rows={shown}
        columns={columns}
        rowKey={(callout) => callout.token}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ningún callout cumple estos filtros.</p>}
      />
    </div>
  );
}
