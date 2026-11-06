'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Bookmark, Pencil, Play, PlugZap, Swords } from 'lucide-react';
import { deleteLibraryCollection, isLibraryNotFound, isLibraryOffline, listLibraryCollections } from '@/lib/api/library';
import type { LibraryCollection } from '@/lib/api/collections';
import { replayHref } from '@/lib/analysis/replay-link';
import { MATCHES_HREF, collectionHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { FILTER_ANY, SEARCH_FILTER, filterRows } from '@/lib/records/table-view';
import { collectionRefs, momentCountLabel } from '@/lib/replay/bookmarks';
import { DataTable, type DataTableColumn } from '@/components/records/data-table';
import { EmptyState } from '@/components/records/empty-state';
import { FilterBar } from '@/components/records/filter-bar';
import { useTableView } from '@/components/records/use-table-view';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PackageImportButton } from '@/components/share/package-import-button';
import { RenameCollectionDialog } from './rename-collection-dialog';

/** A collection already gone counts as deleted: the row only has to leave the list. */
async function deleteCollection(collectionId: string): Promise<void> {
  try {
    await deleteLibraryCollection(collectionId);
  } catch (error) {
    if (!isLibraryNotFound(error)) throw error;
  }
}

/** Colecciones: every saved list of moments, newest edit first. */
export function CollectionLibrary(): ReactNode {
  const [collections, setCollections] = useState<LibraryCollection[] | null>(null);
  const [offline, setOffline] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [renaming, setRenaming] = useState<LibraryCollection | null>(null);
  const view = useTableView('colecciones', {
    sort: { column: 'updated', direction: 'desc' },
    hidden: [],
    filters: { [SEARCH_FILTER]: FILTER_ANY },
  });

  useEffect(() => {
    let active = true;
    listLibraryCollections()
      .then((list) => {
        if (!active) return;
        setCollections(list);
        setOffline(false);
      })
      .catch((error: unknown) => {
        if (!active) return;
        setCollections([]);
        setOffline(isLibraryOffline(error));
      });
    return () => {
      active = false;
    };
  }, [attempt]);

  const reload = (): void => setAttempt((n) => n + 1);

  if (collections === null) {
    return (
      <div role="status" aria-label="Cargando colecciones" className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-xl" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  if (collections.length === 0) {
    return offline ? (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudieron leer las colecciones. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    ) : (
      <EmptyState
        icon={Bookmark}
        title="Aún no hay colecciones"
        description="Abre una partida, ve a la repetición y pulsa Guardar momento en una jugada que quieras volver a ver. Si un compañero te pasó un paquete .cliphub, ábrelo con Importar paquete."
        actions={
          <>
            <Button asChild size="sm">
              <Link href={MATCHES_HREF}>
                <Swords aria-hidden />
                Ver partidas
              </Link>
            </Button>
            <PackageImportButton onImported={reload} />
          </>
        }
      />
    );
  }

  const columns: DataTableColumn<LibraryCollection>[] = [
    { id: 'name', header: 'Colección', required: true, cell: (row) => row.name, sortValue: (row) => row.name },
    { id: 'items', header: 'Momentos', align: 'end', cell: (row) => row.items.length, sortValue: (row) => row.items.length },
    {
      id: 'updated',
      header: 'Última edición',
      cell: (row) => <span className="text-fg-2">{formatShortDate(row.updated_at)}</span>,
      sortValue: (row) => Date.parse(row.updated_at) || null,
    },
    {
      id: 'actions',
      header: '',
      required: true,
      className: 'w-px whitespace-nowrap',
      cell: (row) => (
        <span className="flex items-center justify-end gap-1">
          {row.items.length === 0 ? null : (
            <Button asChild size="sm" variant="outline">
              <Link href={replayHref(collectionRefs(row.items), row.name)}>
                <Play aria-hidden />
                Reproducir
              </Link>
            </Button>
          )}
          <Button type="button" size="icon-sm" variant="ghost" aria-label={`Renombrar ${row.name}`} onClick={() => setRenaming(row)}>
            <Pencil aria-hidden />
          </Button>
          <DeleteMatchButton
            label={row.name}
            confirmText={`¿Borrar ${momentCountLabel(row.items.length)}?`}
            onConfirm={() => deleteCollection(row.id)}
            onDeleted={reload}
          />
        </span>
      ),
    },
  ];

  const shown = filterRows(collections, view.state.filters, { fields: {}, searchText: (row) => row.name });

  return (
    <div className="flex flex-col gap-3">
      <FilterBar
        values={view.state.filters}
        onChange={view.setFilter}
        onReset={view.resetFilters}
        searchPlaceholder="Buscar por nombre"
        count={{ shown: shown.length, total: collections.length, noun: collections.length === 1 ? 'colección' : 'colecciones' }}
        className="flex flex-1 flex-wrap items-center gap-2"
      />
      <PackageImportButton onImported={reload} />
      <DataTable
        label="Colecciones"
        rows={shown}
        columns={columns}
        rowKey={(row) => row.id}
        rowHref={(row) => collectionHref(row.id)}
        view={view}
        empty={<p className="text-body-sm text-fg-2">Ninguna colección se llama así.</p>}
      />
      {renaming === null ? null : (
        <RenameCollectionDialog
          collection={renaming}
          onClose={() => setRenaming(null)}
          onRenamed={(next) => setCollections((current) => (current ?? []).map((row) => (row.id === next.id ? next : row)))}
        />
      )}
    </div>
  );
}
