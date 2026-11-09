'use client';

import { useEffect, useRef, useState, type ChangeEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { BookMarked, FileDown, FileUp, Plus, PlugZap, Swords } from 'lucide-react';
import { deleteLibraryPlay, exportStratbook, isLibraryOffline, listLibraryPlays, patchLibraryPlay } from '@/lib/api/library';
import type { CountNoun } from '@/lib/records/selection';
import type { LibretaStatus } from '@/lib/api/libreta';
import { STRATBOOK_FORMAT, STRATBOOK_VERSION, type LibraryPlay } from '@/lib/api/stratbook';
import { MATCHES_HREF, playHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { importLibretaText } from '@/lib/libreta/libreta-file';
import { STATUS_LABEL, visibleByStatus } from '@/lib/libreta/status';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { PLAY_ORIGIN_LABEL, playKindLabel, playShapeLabel, stratbookFileName } from '@/lib/stratbook/plays';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyCell } from '@/components/records/empty-cell';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { Button } from '@/components/ui/button';
import { LibretaBulkActions } from './bulk-actions';
import { NewPlayDialog } from './new-play-dialog';
import { STATUS_FILTER_OPTIONS, StatusBadge } from './status-badge';

const JUGADA: CountNoun = { one: 'jugada', many: 'jugadas', feminine: true };

const NO_TEAM = 'none';

const COLUMNS: readonly DataTableColumn<LibraryPlay>[] = [
  { id: 'title', header: 'Jugada', required: true, cell: (play) => play.title, sortValue: (play) => play.title },
  {
    id: 'team',
    header: 'Equipo',
    cell: (play) =>
      play.team_id ? (
        <Link href={teamHref(play.team_id)} className="hover:text-primary hover:underline">
          {play.team_name}
        </Link>
      ) : (
        <span className="text-fg-3">Sin equipo</span>
      ),
    sortValue: (play) => play.team_name ?? '',
  },
  { id: 'map', header: 'Mapa', cell: (play) => prettyMapName(play.map), sortValue: (play) => play.map },
  { id: 'side', header: 'Lado', cell: (play) => play.side, sortValue: (play) => play.side },
  { id: 'kind', header: 'Tipo', cell: (play) => playShapeLabel(play), sortValue: (play) => play.kind },
  { id: 'status', header: 'Estado', cell: (play) => <StatusBadge status={play.status} />, sortValue: (play) => play.status },
  {
    id: 'tags',
    header: 'Etiquetas',
    cell: (play) => (play.tags.length > 0 ? play.tags.join(', ') : <EmptyCell label="Sin etiquetas" />),
    sortValue: (play) => play.tags.join(','),
  },
  { id: 'grenades', header: 'Granadas', align: 'end', cell: (play) => play.grenades.length, sortValue: (play) => play.grenades.length },
  { id: 'examples', header: 'Rondas', align: 'end', cell: (play) => play.examples.length, sortValue: (play) => play.examples.length },
  { id: 'origin', header: 'Origen', cell: (play) => PLAY_ORIGIN_LABEL[play.origin], sortValue: (play) => play.origin },
  {
    id: 'updated',
    header: 'Actualizada',
    cell: (play) => <span className="text-fg-2">{formatShortDate(play.updated_at)}</span>,
    sortValue: (play) => Date.parse(play.updated_at) || null,
  },
];

function options(values: Iterable<[string, string]>): { value: string; label: string }[] {
  return [...new Map(values)].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'));
}

/** Downloads a JSON document as a file. */
export function downloadJson(value: unknown, fileName: string): void {
  const url = URL.createObjectURL(new Blob([JSON.stringify(value, null, 2)], { type: 'application/json;charset=utf-8' }));
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = fileName;
  anchor.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

/** The plays of the Libreta, archived ones included; `reload` reads them again. */
export function useLibretaPlays(): { plays: LibraryPlay[] | null; offline: boolean; reload: () => void } {
  const [plays, setPlays] = useState<LibraryPlay[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let active = true;
    listLibraryPlays({ include_archived: true })
      .then((list) => {
        if (!active) return;
        setPlays(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setPlays([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);
  return { plays, offline, reload: () => setAttempt((n) => n + 1) };
}

/** The Jugadas tab: saved plays with their status and tags, bulk edits, and the file export and import. */
export function PlayList({
  plays,
  offline,
  requestedTeam,
  onReload,
  onImported,
  onDetect,
}: {
  plays: LibraryPlay[] | null;
  offline: boolean;
  requestedTeam?: string;
  onReload: () => void;
  /** After an import, which may also bring lineups, role sheets, calls and callout names. */
  onImported: () => void;
  onDetect: () => void;
}): ReactNode {
  const [message, setMessage] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [creating, setCreating] = useState(false);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const fileRef = useRef<HTMLInputElement>(null);
  const view = useTableView('libreta-plays', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: ['tags', 'origin'],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const { setFilter } = view;

  // ?equipo= opens the list on that team's plays once; the filter is not stable, so a ref guards it.
  const appliedTeam = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (requestedTeam === undefined || appliedTeam.current === requestedTeam) return;
    appliedTeam.current = requestedTeam;
    setFilter('team', requestedTeam);
  }, [requestedTeam, setFilter]);

  async function exportPlays(chosen: readonly LibraryPlay[] | null): Promise<void> {
    setBusy(true);
    setMessage(null);
    try {
      const file = chosen
        ? { format: STRATBOOK_FORMAT, version: STRATBOOK_VERSION, exported_at: new Date().toISOString(), plays: [...chosen] }
        : await exportStratbook({ include_archived: true });
      downloadJson(file, stratbookFileName(new Date()));
      setMessage(file.plays.length === 1 ? 'Exportada 1 jugada.' : `Exportadas ${file.plays.length} jugadas.`);
    } catch {
      setMessage('No se pudieron exportar las jugadas.');
    } finally {
      setBusy(false);
    }
  }

  async function importFile(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (file === undefined) return;
    setBusy(true);
    setMessage(null);
    setMessage(await importLibretaText(await file.text()));
    setBusy(false);
    onImported();
  }

  async function bulkStatus(status: LibretaStatus, ids: readonly string[]): Promise<void> {
    setBusy(true);
    const failed = (await Promise.allSettled(ids.map((id) => patchLibraryPlay(id, { status })))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length} ${ids.length === 1 ? 'jugada pasa' : 'jugadas pasan'} a ${STATUS_LABEL[status]}.` : `${failed} no se pudieron cambiar.`);
    setSelected(new Set());
    setBusy(false);
    onReload();
  }

  async function bulkDelete(ids: readonly string[]): Promise<void> {
    setBusy(true);
    const failed = (await Promise.allSettled(ids.map((id) => deleteLibraryPlay(id)))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length === 1 ? 'Eliminada 1 jugada' : `Eliminadas ${ids.length} jugadas`}.` : `${failed} no se pudieron eliminar.`);
    setSelected(new Set());
    setBusy(false);
    onReload();
  }

  const hasPlays = (plays?.length ?? 0) > 0;
  const fileActions = (
    <>
      <Button type="button" size="sm" disabled={busy} onClick={() => setCreating(true)}>
        <Plus aria-hidden />
        Nueva jugada
      </Button>
      <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => fileRef.current?.click()}>
        <FileUp aria-hidden />
        Importar
      </Button>
      {/* An empty Libreta has nothing to export, so the button only appears once there is. */}
      {hasPlays ? (
        <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => void exportPlays(null)}>
          <FileDown aria-hidden />
          Exportar
        </Button>
      ) : null}
      <input
        ref={fileRef}
        type="file"
        accept=".json,application/json"
        aria-label="Fichero de la Libreta"
        className="sr-only"
        tabIndex={-1}
        onChange={(event) => void importFile(event)}
      />
    </>
  );

  const dialog = creating ? <NewPlayDialog knownMaps={plays?.map((play) => play.map)} onClose={() => setCreating(false)} /> : null;
  const status = message ? (
    <p role="status" className="text-body-sm text-fg-2">
      {message}
    </p>
  ) : null;

  if (plays === null) {
    return (
      <TableSkeleton label="Cargando jugadas" columns={6} filters={6} />
    );
  }
  if (plays.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer la Libreta. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
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
          icon={BookMarked}
          title="Todavía no hay jugadas"
          description="Guarda una ronda desde el Replay de una partida, deja que Studio encuentre las jugadas que un equipo repite, escribe una a mano o importa un fichero."
          preview={[
            'Mapa, lado y tipo de jugada, con sus granadas y posiciones.',
            'Estado (en uso, para practicar, en revisión) y etiquetas.',
            'Rondas de ejemplo que abren el Replay en su tick.',
          ]}
          actions={
            <>
              {fileActions}
              <Button type="button" variant="outline" size="sm" onClick={onDetect}>
                Detectar jugadas repetidas
              </Button>
              <Button asChild variant="ghost" size="sm">
                <Link href={MATCHES_HREF}>
                  <Swords aria-hidden />
                  Ver partidas
                </Link>
              </Button>
            </>
          }
        />
        {dialog}
      </>
    );
  }

  const filtered = filterRows(plays, view.state.filters, {
    fields: {
      team: (play) => play.team_id ?? NO_TEAM,
      map: (play) => play.map,
      side: (play) => play.side,
      kind: (play) => play.kind,
      origin: (play) => play.origin,
    },
    searchText: (play) => `${play.title} ${play.team_name ?? ''} ${play.notes} ${prettyMapName(play.map)} ${play.tags.join(' ')}`,
  });
  const shown = visibleByStatus(filtered, view.state.filters.status ?? FILTER_ANY, FILTER_ANY);
  const hiddenArchived = (view.state.filters.status ?? FILTER_ANY) === FILTER_ANY ? filtered.length - shown.length : 0;
  // Only rows the filters show count as chosen, so a bulk action never reaches a hidden one.
  const chosen = shown.filter((play) => selected.has(play.id));
  const chosenIds = chosen.map((play) => play.id);

  return (
    <div className="flex flex-col gap-3">
      {status}
      {hiddenArchived > 0 ? (
        <p className="text-meta text-fg-3">
          {hiddenArchived === 1 ? 'Una jugada archivada no se muestra.' : `${hiddenArchived} jugadas archivadas no se muestran.`} Elige Estado: Archivo para verlas.
        </p>
      ) : null}
      <DataTable
        toolbar={
          <FilterBar
            values={view.state.filters}
            onChange={view.setFilter}
            onReset={view.resetFilters}
            searchPlaceholder="Buscar por título, notas o etiqueta"
            filters={[
              {
                id: 'team',
                label: 'Equipo',
                options: options(plays.map((play) => [play.team_id ?? NO_TEAM, play.team_name ?? 'Sin equipo'])),
              },
              { id: 'map', label: 'Mapa', options: options(plays.map((play) => [play.map, prettyMapName(play.map)])) },
              { id: 'side', label: 'Lado', options: [{ value: 'T', label: 'T' }, { value: 'CT', label: 'CT' }] },
              { id: 'kind', label: 'Tipo', options: options(plays.map((play) => [play.kind, playKindLabel(play.kind)])) },
              { id: 'status', label: 'Estado', options: STATUS_FILTER_OPTIONS },
              {
                id: 'origin',
                label: 'Origen',
                options: (['round', 'detected', 'import', 'manual', 'playbook'] as const).map((value) => ({ value, label: PLAY_ORIGIN_LABEL[value] })),
              },
            ]}
            count={{ shown: shown.length, total: plays.length, noun: plays.length === 1 ? 'jugada' : 'jugadas' }}
          />
        }
        actions={fileActions}
        label="Jugadas"
        rows={shown}
        columns={COLUMNS}
        rowKey={(play) => play.id}
        rowHref={(play) => playHref(play.id)}
        view={view}
        selection={{
          selected,
          onChange: setSelected,
          busy,
          noun: JUGADA,
          actions: (
            <LibretaBulkActions
              count={chosen.length}
              noun={JUGADA}
              busy={busy}
              onStatus={(next) => void bulkStatus(next, chosenIds)}
              onExport={() => void exportPlays(chosen)}
              onDelete={() => void bulkDelete(chosenIds)}
            />
          ),
        }}
        empty={<p className="text-body-sm text-fg-2">Ninguna jugada cumple estos filtros.</p>}
      />
      {dialog}
    </div>
  );
}
