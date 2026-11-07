'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Crosshair, Plus, PlugZap, Users } from 'lucide-react';
import { GRENADE_KIND_LABEL } from '@/lib/analysis/grenade-patterns';
import { TEAM_GRENADE_KINDS } from '@/lib/api/grenades';
import { isLibraryOffline } from '@/lib/api/library';
import type { LibretaStatus } from '@/lib/api/libreta';
import { deleteLibraryLineup, listLibraryLineups, patchLibraryLineup, type LibraryLineup } from '@/lib/api/lineups';
import { TEAMS_HREF, lineupHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import type { CalloutLabel } from '@/lib/libreta/callouts';
import { lineupSpot, practiceBlockReason, techniqueLabel } from '@/lib/libreta/lineups';
import { STATUS_LABEL, visibleByStatus } from '@/lib/libreta/status';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyCell } from '@/components/records/empty-cell';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { Button } from '@/components/ui/button';
import { useCalloutNames } from '@/hooks/use-callout-names';
import { BulkBar } from './bulk-bar';
import { GrenadeName, TechniqueText } from './lineup-fields';
import { NewLineupDialog } from './new-lineup-dialog';
import { STATUS_FILTER_OPTIONS, StatusBadge } from './status-badge';

function Callout({ text }: { text: string }): ReactNode {
  return text || <span className="text-fg-3">Sin callout</span>;
}

function lineupColumns(calloutLabel: CalloutLabel): DataTableColumn<LibraryLineup>[] {
  return [
    { id: 'title', header: 'Lineup', required: true, cell: (lineup) => lineup.title, sortValue: (lineup) => lineup.title },
    { id: 'map', header: 'Mapa', cell: (lineup) => prettyMapName(lineup.map), sortValue: (lineup) => lineup.map },
    { id: 'side', header: 'Lado', cell: (lineup) => lineup.side, sortValue: (lineup) => lineup.side },
    { id: 'grenade', header: 'Granada', cell: (lineup) => <GrenadeName kind={lineup.grenade} />, sortValue: (lineup) => lineup.grenade },
    {
      id: 'from',
      header: 'Desde',
      cell: (lineup) => <Callout text={calloutLabel(lineup.map, lineup.throw_callout)} />,
      sortValue: (lineup) => calloutLabel(lineup.map, lineup.throw_callout),
    },
    {
      id: 'landing',
      header: 'Cae en',
      cell: (lineup) => <Callout text={calloutLabel(lineup.map, lineup.landing_callout)} />,
      sortValue: (lineup) => calloutLabel(lineup.map, lineup.landing_callout),
    },
    { id: 'technique', header: 'Técnica', cell: (lineup) => <TechniqueText lineup={lineup} />, sortValue: (lineup) => techniqueLabel(lineup) },
    { id: 'status', header: 'Estado', cell: (lineup) => <StatusBadge status={lineup.status} />, sortValue: (lineup) => lineup.status },
    {
      id: 'tags',
      header: 'Etiquetas',
      cell: (lineup) => (lineup.tags.length > 0 ? lineup.tags.join(', ') : <EmptyCell label="Sin etiquetas" />),
      sortValue: (lineup) => lineup.tags.join(','),
    },
    {
      id: 'updated',
      header: 'Actualizado',
      cell: (lineup) => <span className="text-fg-2">{formatShortDate(lineup.updated_at)}</span>,
      sortValue: (lineup) => Date.parse(lineup.updated_at) || null,
    },
  ];
}

/** The lineups of the Libreta, archived ones included; `reload` reads them again. */
export function useLibretaLineups(): { lineups: LibraryLineup[] | null; offline: boolean; reload: () => void } {
  const [lineups, setLineups] = useState<LibraryLineup[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    listLibraryLineups({ include_archived: true })
      .then((list) => {
        if (!active) return;
        setLineups(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setLineups([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);
  return { lineups, offline, reload: () => setAttempt((n) => n + 1) };
}

/** The Lineups tab: grenades saved with where to stand and aim, practised in CS2 a map at a time. */
export function LineupList({
  lineups,
  offline,
  onReload,
}: {
  lineups: LibraryLineup[] | null;
  offline: boolean;
  onReload: () => void;
}): ReactNode {
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const view = useTableView('libreta-lineups', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: ['tags'],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const calloutLabel = useCalloutNames((lineups ?? []).map((lineup) => lineup.map));
  const columns = useMemo(() => lineupColumns(calloutLabel), [calloutLabel]);
  const filtered = filterRows(lineups ?? [], view.state.filters, {
    fields: {
      map: (lineup) => lineup.map,
      side: (lineup) => lineup.side,
      grenade: (lineup) => lineup.grenade,
    },
    searchText: (lineup) =>
      [
        lineup.title,
        calloutLabel(lineup.map, lineup.throw_callout),
        calloutLabel(lineup.map, lineup.landing_callout),
        lineup.tags.join(' '),
        lineup.notes,
        prettyMapName(lineup.map),
      ].join(' '),
  });
  const shown = visibleByStatus(filtered, view.state.filters.status ?? FILTER_ANY, FILTER_ANY);
  const hiddenArchived = (view.state.filters.status ?? FILTER_ANY) === FILTER_ANY ? filtered.length - shown.length : 0;
  // Only rows the filters show count as chosen, so a bulk action never reaches a hidden one.
  const chosen = shown.filter((lineup) => selected.has(lineup.id));
  const practice = usePracticeLaunch(chosen[0]?.map ?? '');
  const blocked = practiceBlockReason(chosen);

  async function bulkStatus(status: LibretaStatus): Promise<void> {
    setBusy(true);
    const ids = chosen.map((lineup) => lineup.id);
    const failed = (await Promise.allSettled(ids.map((id) => patchLibraryLineup(id, { status })))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length} ${ids.length === 1 ? 'lineup pasa' : 'lineups pasan'} a ${STATUS_LABEL[status]}.` : `${failed} no se pudieron cambiar.`);
    setSelected(new Set());
    setBusy(false);
    onReload();
  }

  async function bulkDelete(): Promise<void> {
    setBusy(true);
    const ids = chosen.map((lineup) => lineup.id);
    const failed = (await Promise.allSettled(ids.map((id) => deleteLibraryLineup(id)))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length === 1 ? 'Eliminado 1 lineup' : `Eliminados ${ids.length} lineups`}.` : `${failed} no se pudieron eliminar.`);
    setSelected(new Set());
    setBusy(false);
    onReload();
  }

  const newButton = (
    <Button type="button" size="sm" disabled={busy} onClick={() => setCreating(true)}>
      <Plus aria-hidden />
      Nuevo lineup
    </Button>
  );
  const dialog = creating ? <NewLineupDialog knownMaps={lineups?.map((lineup) => lineup.map)} onClose={() => setCreating(false)} /> : null;
  const status = message ? (
    <p role="status" className="text-body-sm text-fg-2">
      {message}
    </p>
  ) : null;

  if (lineups === null) {
    return (
      <TableSkeleton label="Cargando lineups" columns={5} filters={4} />
    );
  }
  if (lineups.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer los lineups. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={onReload}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <>
        {status}
        <EmptyState
          icon={Crosshair}
          title="Todavía no hay lineups"
          description="Abre un equipo en Equipos y, en su pestaña Granadas, pulsa Guardar lineup en la granada que quieras. También puedes crear uno pegando lo que escribe getpos en la consola de CS2."
          preview={[
            'Dónde ponerte y adónde apuntar, listo para copiar en la consola.',
            'Cómo se lanza: parado o corriendo, con salto o sin él, y con qué clic.',
            'Practicar en CS2 con la granada en la mano, hasta 20 del mismo mapa a la vez.',
          ]}
          actions={
            <>
              <Button asChild variant="outline" size="sm">
                <Link href={TEAMS_HREF}>
                  <Users aria-hidden />
                  Ver equipos
                </Link>
              </Button>
              {newButton}
            </>
          }
        />
        {dialog}
      </>
    );
  }

  const maps = [...new Set(lineups.map((lineup) => lineup.map))]
    .map((map) => ({ value: map, label: prettyMapName(map) }))
    .sort((a, b) => a.label.localeCompare(b.label, 'es'));

  return (
    <div className="flex flex-col gap-3">
      {status}
      {hiddenArchived > 0 ? (
        <p className="text-meta text-fg-3">
          {hiddenArchived === 1 ? 'Un lineup archivado no se muestra.' : `${hiddenArchived} lineups archivados no se muestran.`} Elige Estado: Archivo para verlos.
        </p>
      ) : null}
      {chosen.length > 0 ? (
        <BulkBar
          count={chosen.length}
          noun={chosen.length === 1 ? 'lineup' : 'lineups'}
          busy={busy}
          onStatus={(next) => void bulkStatus(next)}
          onDelete={() => void bulkDelete()}
          onClear={() => setSelected(new Set())}
          extra={
            <span title={blocked ?? undefined}>
              <Button
                type="button"
                variant="outline"
                size="xs"
                disabled={busy || blocked !== null || practice.state.kind === 'busy'}
                onClick={() => practice.launch(chosen.map(lineupSpot))}
              >
                <Crosshair aria-hidden />
                Practicar en CS2
              </Button>
            </span>
          }
        />
      ) : null}
      <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
      <DataTable
        toolbar={
          <FilterBar
            values={view.state.filters}
            onChange={view.setFilter}
            onReset={view.resetFilters}
            searchPlaceholder="Buscar por título, callout, etiqueta o notas"
            filters={[
              { id: 'map', label: 'Mapa', options: maps },
              { id: 'side', label: 'Lado', options: [{ value: 'T', label: 'T' }, { value: 'CT', label: 'CT' }] },
              { id: 'grenade', label: 'Granada', options: TEAM_GRENADE_KINDS.map((kind) => ({ value: kind, label: GRENADE_KIND_LABEL[kind] })) },
              { id: 'status', label: 'Estado', options: STATUS_FILTER_OPTIONS },
            ]}
            count={{ shown: shown.length, total: lineups.length, noun: lineups.length === 1 ? 'lineup' : 'lineups' }}
          />
        }
        actions={newButton}
        label="Lineups"
        rows={shown}
        columns={columns}
        rowKey={(lineup) => lineup.id}
        rowHref={(lineup) => lineupHref(lineup.id)}
        view={view}
        selection={{ selected, onChange: setSelected }}
        empty={<p className="text-body-sm text-fg-2">Ningún lineup cumple estos filtros.</p>}
      />
      {dialog}
    </div>
  );
}
