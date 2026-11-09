'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type ReactNode } from 'react';
import { FileDown, FileUp, ListVideo, PenLine, Plus, PlugZap, Star } from 'lucide-react';
import { isLibraryOffline } from '@/lib/api/library';
import { createPlaylist, deletePlaylist, exportPlaylists, importPlaylists, listPlaylists, patchPlaylist, type SavedPlaylist } from '@/lib/api/playlists';
import { formatShortDate, prettyMapName } from '@/lib/format';
import type { CountNoun } from '@/lib/records/selection';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { missingRounds, onShelf, parsePlaylistFile, playlistFileName, playlistFolders, playlistImportMessage, playlistSearchText, type PlaylistShelf } from '@/lib/replay/playlist-library';
import { savedPlaylistHref } from '@/lib/replay/saved-playlist';
import { downloadJson } from '@/components/libreta/play-list';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { DeleteChosenButton } from '@/components/records/delete-chosen-button';
import { EmptyCell } from '@/components/records/empty-cell';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { TableSkeleton } from '@/components/records/table-skeleton';
import { useTableView } from '@/components/records/use-table-view';
import { Button, FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { PlaylistDialog, type PlaylistDetails } from './playlist-dialog';
import { usePlaylistChanges } from './use-saved-playlists';

const PLAYLIST: CountNoun = { one: 'playlist', many: 'playlists', feminine: true };
const ALL: PlaylistShelf = { kind: 'all' };

function sameShelf(a: PlaylistShelf, b: PlaylistShelf): boolean {
  if (a.kind === 'folder' && b.kind === 'folder') return a.name.toLocaleLowerCase('es') === b.name.toLocaleLowerCase('es');
  return a.kind === b.kind;
}

function playlistColumns(input: { onStar: (p: SavedPlaylist) => void; onEdit: (p: SavedPlaylist) => void }): DataTableColumn<SavedPlaylist>[] {
  return [
    { id: 'name', header: 'Nombre', required: true, cell: (p) => p.name, sortValue: (p) => p.name },
    {
      id: 'favorite',
      header: 'Favorita',
      cell: (p) => (
        <Button
          type="button"
          size="icon-xs"
          variant="ghost"
          aria-pressed={p.favorite}
          aria-label={p.favorite ? `Quitar «${p.name}» de favoritas` : `Marcar «${p.name}» como favorita`}
          onClick={() => input.onStar(p)}
        >
          <Star aria-hidden className={p.favorite ? 'fill-primary text-primary' : undefined} />
        </Button>
      ),
      sortValue: (p) => (p.favorite ? 0 : 1),
    },
    { id: 'folder', header: 'Carpeta', cell: (p) => p.folder || <EmptyCell label="Sin carpeta" />, sortValue: (p) => p.folder || null },
    {
      id: 'maps',
      header: 'Mapas',
      cell: (p) => (p.maps.length > 0 ? p.maps.map(prettyMapName).join(', ') : <EmptyCell label="Sin rondas" />),
      sortValue: (p) => p.maps.map(prettyMapName).join(', ') || null,
    },
    {
      id: 'rounds',
      header: 'Rondas',
      align: 'end',
      cell: (p) => {
        const missing = missingRounds(p);
        return (
          <span className="tabular-nums">
            {p.rounds.length}
            {missing > 0 ? <span className="pl-1.5 text-meta text-fg-3">{missing} sin demo</span> : null}
          </span>
        );
      },
      sortValue: (p) => p.rounds.length,
    },
    { id: 'author', header: 'Autor', cell: (p) => p.author || <EmptyCell label="Sin autor" />, sortValue: (p) => p.author || null },
    {
      id: 'updated',
      header: 'Actualizada',
      cell: (p) => <span className="text-fg-2">{formatShortDate(p.updated_at)}</span>,
      sortValue: (p) => Date.parse(p.updated_at) || null,
    },
    {
      id: 'edit',
      header: 'Editar',
      cell: (p) => (
        <Button type="button" size="icon-xs" variant="ghost" aria-label={`Editar «${p.name}»`} title="Nombre, carpeta y autor" onClick={() => input.onEdit(p)}>
          <PenLine aria-hidden />
        </Button>
      ),
    },
  ];
}

/** Every saved playlist, read again when one changes in this window. */
function usePlaylists(): { playlists: SavedPlaylist[] | null; offline: boolean; reload: () => void } {
  const [playlists, setPlaylists] = useState<SavedPlaylist[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const reload = useCallback(() => setAttempt((n) => n + 1), []);
  usePlaylistChanges(reload);
  useEffect(() => {
    let active = true;
    listPlaylists()
      .then((list) => {
        if (!active) return;
        setPlaylists(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setPlaylists((current) => current ?? []);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);
  return { playlists, offline, reload };
}

type Editing = { kind: 'new' } | { kind: 'edit'; playlist: SavedPlaylist } | null;

/** Replay > Playlists: the team's saved rounds by folder, to open in the Replay or share as a file. */
export function PlaylistLibrary(): ReactNode {
  const { playlists, offline, reload } = usePlaylists();
  const [shelf, setShelf] = useState<PlaylistShelf>(ALL);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [editing, setEditing] = useState<Editing>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const view = useTableView('replay-playlists', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });
  const columns = useMemo(
    () =>
      playlistColumns({
        onStar: (p) => {
          patchPlaylist(p.id, { favorite: !p.favorite })
            .then(reload)
            .catch(() => setMessage('No se pudo cambiar la favorita. Comprueba que Studio sigue abierto.'));
        },
        onEdit: (p) => setEditing({ kind: 'edit', playlist: p }),
      }),
    [reload],
  );

  const all = playlists ?? [];
  const folders = playlistFolders(all);
  const onThisShelf = all.filter((p) => onShelf(p, shelf));
  const shown = filterRows(onThisShelf, view.state.filters, { fields: { map: (p) => p.maps }, searchText: playlistSearchText });
  const chosenIds = shown.filter((p) => selected.has(p.id)).map((p) => p.id);
  const mapOptions = [...new Set(all.flatMap((p) => p.maps))]
    .map((map) => ({ value: map, label: prettyMapName(map) }))
    .sort((a, b) => a.label.localeCompare(b.label, 'es'));

  async function save(details: PlaylistDetails): Promise<string | null> {
    try {
      if (editing?.kind === 'edit') await patchPlaylist(editing.playlist.id, details);
      else await createPlaylist({ ...details, rounds: [] });
    } catch {
      return 'No se pudo guardar la playlist. Comprueba que Studio sigue abierto.';
    }
    setMessage(editing?.kind === 'edit' ? null : `«${details.name}» está lista. Abre una partida en el Replay y pulsa B o «Añadir a playlist» en cada ronda que quieras.`);
    reload();
    return null;
  }

  async function exportShown(ids: readonly string[]): Promise<void> {
    setBusy(true);
    try {
      const file = await exportPlaylists(ids);
      downloadJson(file, playlistFileName(new Date()));
      setMessage(`${file.playlists.length === 1 ? 'Exportada 1 playlist' : `Exportadas ${file.playlists.length} playlists`}. Pasa el fichero al equipo y que lo importe aquí.`);
    } catch {
      setMessage('No se pudieron exportar las playlists. Comprueba que Studio sigue abierto.');
    } finally {
      setBusy(false);
    }
  }

  async function importFile(event: ChangeEvent<HTMLInputElement>): Promise<void> {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (file === undefined) return;
    setBusy(true);
    const parsed = parsePlaylistFile(await file.text());
    if (!parsed.ok) {
      setMessage(parsed.error);
      setBusy(false);
      return;
    }
    try {
      setMessage(playlistImportMessage(await importPlaylists(parsed.file)));
    } catch {
      setMessage('No se pudo importar el fichero. Comprueba que Studio sigue abierto.');
    }
    setBusy(false);
    reload();
  }

  async function removeChosen(ids: readonly string[]): Promise<void> {
    setBusy(true);
    const failed = (await Promise.allSettled(ids.map((id) => deletePlaylist(id)))).filter((r) => r.status === 'rejected').length;
    setMessage(failed === 0 ? `${ids.length === 1 ? 'Borrada 1 playlist' : `Borradas ${ids.length} playlists`}.` : `${failed} no se pudieron borrar.`);
    setSelected(new Set());
    setBusy(false);
    reload();
  }

  const newButton = (
    <Button type="button" size="sm" disabled={busy} onClick={() => setEditing({ kind: 'new' })}>
      <Plus aria-hidden />
      Nueva playlist
    </Button>
  );
  const importButton = (
    <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => fileRef.current?.click()}>
      <FileUp aria-hidden />
      Importar
    </Button>
  );
  const fileInput = (
    <input
      ref={fileRef}
      type="file"
      accept=".json,application/json"
      aria-label="Fichero de playlists"
      className="sr-only"
      tabIndex={-1}
      onChange={(event) => void importFile(event)}
    />
  );
  const dialog =
    editing === null ? null : (
      <PlaylistDialog
        key={editing.kind === 'edit' ? editing.playlist.id : 'new'}
        title={editing.kind === 'edit' ? 'Editar la playlist' : 'Nueva playlist'}
        description={editing.kind === 'edit' ? 'Las rondas y sus comentarios no cambian.' : 'Después suma rondas desde el Replay de cualquier partida.'}
        submitLabel={editing.kind === 'edit' ? 'Guardar' : 'Crear playlist'}
        initial={editing.kind === 'edit' ? { name: editing.playlist.name, folder: editing.playlist.folder, author: editing.playlist.author } : undefined}
        folders={folders.map((f) => f.name)}
        onSubmit={save}
        onClose={() => setEditing(null)}
      />
    );
  const status = message ? (
    <p role="status" className="text-body-sm text-fg-2">
      {message}
    </p>
  ) : null;

  if (playlists === null) return <TableSkeleton label="Cargando playlists" columns={5} filters={2} />;
  if (playlists.length === 0) {
    if (offline) {
      return (
        <EmptyState
          icon={PlugZap}
          title="Servicio local sin conexión"
          description="No se pudieron leer las playlists. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
          actions={
            <Button type="button" variant="outline" size="sm" onClick={reload}>
              Reintentar
            </Button>
          }
        />
      );
    }
    return (
      <>
        {status}
        <EmptyState
          icon={ListVideo}
          title="Aún no hay playlists"
          description="Junta las rondas que quieres enseñar al equipo, de cualquier partida, en el orden en que las vais a ver."
          preview={[
            'Mientras ves una partida en el Replay, pulsa B o «Añadir a playlist» en cada ronda.',
            'En la reunión pasáis de ronda con J y K, con el comentario de cada una a la izquierda.',
            'Exporta la playlist a un fichero para que el equipo la abra en su Studio.',
          ]}
          actions={
            <>
              {newButton}
              {importButton}
            </>
          }
        />
        {fileInput}
        {dialog}
      </>
    );
  }

  const exportIds = chosenIds.length > 0 ? chosenIds : shown.map((p) => p.id);
  return (
    <div className="grid gap-4 lg:grid-cols-[13rem_minmax(0,1fr)]">
      <ShelfList
        shelf={shelf}
        onShelf={(next) => {
          setShelf(next);
          setSelected(new Set());
        }}
        counts={{ all: all.length, favorites: all.filter((p) => p.favorite).length, none: all.filter((p) => p.folder === '').length }}
        folders={folders}
      />
      <div className="flex min-w-0 flex-col gap-3">
        {status}
        <DataTable
          toolbar={
            <FilterBar
              values={view.state.filters}
              onChange={view.setFilter}
              onReset={view.resetFilters}
              searchPlaceholder="Buscar por nombre, autor o carpeta"
              filters={[{ id: 'map', label: 'Mapa', options: mapOptions }]}
              count={{ shown: shown.length, total: onThisShelf.length, noun: onThisShelf.length === 1 ? 'playlist' : 'playlists' }}
            />
          }
          actions={
            <>
              {importButton}
              <Button type="button" variant="outline" size="sm" disabled={busy || exportIds.length === 0} onClick={() => void exportShown(exportIds)}>
                <FileDown aria-hidden />
                Exportar al equipo
              </Button>
              {newButton}
            </>
          }
          label="Playlists"
          rows={shown}
          columns={columns}
          rowKey={(p) => p.id}
          rowHref={(p) => savedPlaylistHref(p.id)}
          view={view}
          selection={{
            selected,
            onChange: setSelected,
            busy,
            noun: PLAYLIST,
            actions: (
              <>
                <Button type="button" variant="outline" size="xs" disabled={busy} onClick={() => void exportShown(chosenIds)}>
                  <FileDown aria-hidden />
                  Exportar al equipo
                </Button>
                <DeleteChosenButton
                  count={chosenIds.length}
                  noun={PLAYLIST}
                  loading={busy}
                  description="Las partidas siguen en la biblioteca; se pierden el orden de las rondas, sus comentarios y sus dibujos. No se puede deshacer."
                  onConfirm={() => void removeChosen(chosenIds)}
                />
              </>
            ),
          }}
          empty={<p className="text-body-sm text-fg-2">Ninguna playlist de esta carpeta cumple estos filtros.</p>}
        />
        {fileInput}
        {dialog}
      </div>
    </div>
  );
}

function ShelfList({
  shelf,
  onShelf: pick,
  counts,
  folders,
}: {
  shelf: PlaylistShelf;
  onShelf: (next: PlaylistShelf) => void;
  counts: { all: number; favorites: number; none: number };
  folders: readonly { name: string; count: number }[];
}): ReactNode {
  const item = (target: PlaylistShelf, label: string, n: number): ReactNode => {
    const current = sameShelf(shelf, target);
    return (
      <li key={target.kind === 'folder' ? `folder:${target.name}` : target.kind}>
        <button
          type="button"
          aria-current={current ? 'true' : undefined}
          onClick={() => pick(target)}
          className={cn(
            'flex w-full items-baseline justify-between gap-2 rounded-sm px-2 py-1.5 text-left text-body-sm outline-none',
            current ? 'bg-surface-3 font-medium text-primary' : 'text-fg-2 hover:bg-surface-2 hover:text-fg-1',
            FOCUS_RING,
          )}
        >
          <span className="truncate">{label}</span>
          <span className="tabular-nums text-meta text-fg-3">{n}</span>
        </button>
      </li>
    );
  };
  return (
    <nav aria-label="Carpetas de playlists" className="flex flex-col gap-3">
      <ul className="flex flex-col gap-0.5">
        {item(ALL, 'Todas', counts.all)}
        {item({ kind: 'favorites' }, 'Favoritas', counts.favorites)}
        {item({ kind: 'none' }, 'Sin carpeta', counts.none)}
      </ul>
      {folders.length === 0 ? (
        <p className="px-2 text-meta text-fg-3">Pon una carpeta al crear o editar una playlist, como Rivales o Pretemporada.</p>
      ) : (
        <section aria-label="Carpetas" className="flex flex-col gap-1">
          <h3 className="px-2 text-meta text-fg-3">Carpetas</h3>
          <ul className="flex flex-col gap-0.5">{folders.map((f) => item({ kind: 'folder', name: f.name }, f.name, f.count))}</ul>
        </section>
      )}
    </nav>
  );
}
