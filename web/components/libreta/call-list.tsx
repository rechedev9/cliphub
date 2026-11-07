'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { Megaphone, Plus, PlugZap } from 'lucide-react';
import { deleteLibraryCall, listLibraryCalls, patchLibraryCall, type LibraryCall } from '@/lib/api/calls';
import { isLibraryOffline } from '@/lib/api/library';
import type { LibretaStatus } from '@/lib/api/libreta';
import { formatShortDate } from '@/lib/format';
import { CALL_FILTER_FIELDS, CALL_SIDE_LABEL, callSearchText, tagOptions } from '@/lib/libreta/calls';
import { STATUS_LABEL, visibleByStatus } from '@/lib/libreta/status';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { BulkBar } from './bulk-bar';
import { CallDialog } from './call-dialog';
import { STATUS_FILTER_OPTIONS, StatusBadge } from './status-badge';

const SIDE_OPTIONS = [
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
  { value: 'both', label: 'Ambos' },
];

function callColumns(onEdit: (call: LibraryCall) => void): DataTableColumn<LibraryCall>[] {
  return [
    {
      id: 'name',
      header: 'Llamada',
      required: true,
      cell: (call) => (
        <button
          type="button"
          onClick={() => onEdit(call)}
          className="rounded-sm text-left font-medium text-fg-1 underline-offset-4 outline-none hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
        >
          {call.name}
        </button>
      ),
      sortValue: (call) => call.name,
    },
    { id: 'side', header: 'Lado', cell: (call) => CALL_SIDE_LABEL[call.side], sortValue: (call) => call.side },
    {
      id: 'meaning',
      header: 'Qué significa',
      cell: (call) =>
        call.meaning ? (
          <span className="line-clamp-2 max-w-xl whitespace-pre-line text-fg-2">{call.meaning}</span>
        ) : (
          <span className="text-fg-3">Sin escribir</span>
        ),
      sortValue: (call) => call.meaning,
    },
    { id: 'status', header: 'Estado', cell: (call) => <StatusBadge status={call.status} />, sortValue: (call) => call.status },
    {
      id: 'tags',
      header: 'Etiquetas',
      cell: (call) => (call.tags.length > 0 ? call.tags.join(', ') : <span className="text-fg-3">Ninguna</span>),
      sortValue: (call) => call.tags.join(','),
    },
    {
      id: 'updated',
      header: 'Actualizada',
      cell: (call) => <span className="text-fg-2">{formatShortDate(call.updated_at)}</span>,
      sortValue: (call) => Date.parse(call.updated_at) || null,
    },
  ];
}

/** The calls of the Libreta, archived ones included; `reload` reads them again. */
export function useLibretaCalls(): { calls: LibraryCall[] | null; offline: boolean; reload: () => void } {
  const [calls, setCalls] = useState<LibraryCall[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    listLibraryCalls({ include_archived: true })
      .then((list) => {
        if (!active) return;
        setCalls(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setCalls([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);
  return { calls, offline, reload: () => setAttempt((n) => n + 1) };
}

/** Which dialog is open: a new call, or the call being edited. */
type Editing = { kind: 'new' } | { kind: 'edit'; call: LibraryCall } | null;

/** The Llamadas tab: the team's words for the game and what each one asks for. */
export function CallList({ calls, offline, onReload }: { calls: LibraryCall[] | null; offline: boolean; onReload: () => void }): ReactNode {
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [editing, setEditing] = useState<Editing>(null);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const view = useTableView('libreta-calls', {
    sort: { column: 'name', direction: 'asc' },
    hidden: ['updated'],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const columns = useMemo(() => callColumns((call) => setEditing({ kind: 'edit', call })), []);

  async function bulkStatus(status: LibretaStatus, ids: readonly string[]): Promise<void> {
    setBusy(true);
    const failed = (await Promise.allSettled(ids.map((id) => patchLibraryCall(id, { status })))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length} ${ids.length === 1 ? 'llamada pasa' : 'llamadas pasan'} a ${STATUS_LABEL[status]}.` : `${failed} no se pudieron cambiar.`);
    setSelected(new Set());
    setBusy(false);
    onReload();
  }

  async function bulkDelete(ids: readonly string[]): Promise<void> {
    setBusy(true);
    const failed = (await Promise.allSettled(ids.map((id) => deleteLibraryCall(id)))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length === 1 ? 'Eliminada 1 llamada' : `Eliminadas ${ids.length} llamadas`}.` : `${failed} no se pudieron eliminar.`);
    setSelected(new Set());
    setBusy(false);
    onReload();
  }

  const newButton = (
    <Button type="button" size="sm" disabled={busy} onClick={() => setEditing({ kind: 'new' })}>
      <Plus aria-hidden />
      Nueva llamada
    </Button>
  );
  const dialog = editing ? (
    <CallDialog
      key={editing.kind === 'edit' ? editing.call.id : 'new'}
      call={editing.kind === 'edit' ? editing.call : undefined}
      onClose={() => setEditing(null)}
      onSaved={() => {
        setMessage(null);
        onReload();
      }}
    />
  ) : null;
  const status = message ? (
    <p role="status" className="text-body-sm text-fg-2">
      {message}
    </p>
  ) : null;

  if (calls === null) {
    return (
      <div role="status" aria-label="Cargando llamadas" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (calls.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer las llamadas. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
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
          icon={Megaphone}
          title="Todavía no hay llamadas"
          description="Apunta las palabras que dice tu equipo en la partida y qué tiene que hacer cada uno al oírlas, para que todos entiendan lo mismo."
          preview={[
            'El nombre de la llamada y lo que pide, con todo el detalle que haga falta.',
            'Si es de T, de CT o de los dos lados.',
            'Estado y etiquetas, como en las jugadas y los lineups.',
          ]}
          actions={newButton}
        />
        {dialog}
      </>
    );
  }

  const filtered = filterRows(calls, view.state.filters, { fields: CALL_FILTER_FIELDS, searchText: callSearchText });
  const shown = visibleByStatus(filtered, view.state.filters.status ?? FILTER_ANY, FILTER_ANY);
  const hiddenArchived = (view.state.filters.status ?? FILTER_ANY) === FILTER_ANY ? filtered.length - shown.length : 0;
  // Only rows the filters show count as chosen, so a bulk action never reaches a hidden one.
  const chosen = shown.filter((call) => selected.has(call.id));
  const chosenIds = chosen.map((call) => call.id);

  return (
    <div className="flex flex-col gap-3">
      {status}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <FilterBar
          values={view.state.filters}
          onChange={view.setFilter}
          onReset={view.resetFilters}
          searchPlaceholder="Buscar por llamada, significado o etiqueta"
          filters={[
            { id: 'side', label: 'Lado', options: SIDE_OPTIONS },
            { id: 'status', label: 'Estado', options: STATUS_FILTER_OPTIONS },
            { id: 'tag', label: 'Etiqueta', options: tagOptions(calls) },
          ]}
          count={{ shown: shown.length, total: calls.length, noun: calls.length === 1 ? 'llamada' : 'llamadas' }}
          className="flex flex-1 flex-wrap items-center gap-2"
        />
        <div className="flex flex-wrap gap-2">{newButton}</div>
      </div>
      {hiddenArchived > 0 ? (
        <p className="text-meta text-fg-3">
          {hiddenArchived === 1 ? 'Una llamada archivada no se muestra.' : `${hiddenArchived} llamadas archivadas no se muestran.`} Elige Estado: Archivo para verlas.
        </p>
      ) : null}
      {chosen.length > 0 ? (
        <BulkBar
          count={chosen.length}
          noun={chosen.length === 1 ? 'llamada' : 'llamadas'}
          busy={busy}
          onStatus={(next) => void bulkStatus(next, chosenIds)}
          onDelete={() => void bulkDelete(chosenIds)}
          onClear={() => setSelected(new Set())}
        />
      ) : null}
      <DataTable
        label="Llamadas"
        rows={shown}
        columns={columns}
        rowKey={(call) => call.id}
        view={view}
        selection={{ selected, onChange: setSelected }}
        empty={<p className="text-body-sm text-fg-2">Ninguna llamada cumple estos filtros.</p>}
      />
      {dialog}
    </div>
  );
}
