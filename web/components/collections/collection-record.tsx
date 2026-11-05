'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ArrowDown, ArrowUp, Bookmark, Pencil, Play, PlugZap, Smartphone, Trash2, TriangleAlert } from 'lucide-react';
import {
  deleteLibraryCollection,
  getLibraryCollection,
  isLibraryNotFound,
  isLibraryOffline,
  listLibraryMatches,
  patchLibraryCollection,
} from '@/lib/api/library';
import type { LibraryMatch } from '@/lib/api/library';
import { COLLECTION_NOTE_MAX } from '@/lib/api/collections';
import type { CollectionItem, LibraryCollectionDetail } from '@/lib/api/collections';
import { replayHref } from '@/lib/analysis/replay-link';
import { COLLECTIONS_HREF } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { groupEvidenceByMatch } from '@/lib/library/analysis-video';
import { matchTitle } from '@/lib/library/partidas';
import { collectionRefs, momentCountLabel, moveItem, playableCount, removeItem, setItemNote } from '@/lib/replay/bookmarks';
import { AnalysisVideoDialog } from '@/components/analysis-video/analysis-video-dialog';
import { DeleteMatchButton } from '@/components/matches/delete-match-button';
import { EmptyState } from '@/components/records/empty-state';
import { RecordHeader } from '@/components/records/record-header';
import { NOTE_FIELD_CLASS } from '@/components/replay/bookmark-panel';
import { useDocumentTitle, useRouteTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { RenameCollectionDialog } from './rename-collection-dialog';

type Load = { kind: 'loading' } | { kind: 'ready'; collection: LibraryCollectionDetail } | { kind: 'missing' } | { kind: 'offline' };

/** One collection: its moments in play order, with their notes, and the way to a replay or a Short. */
export function CollectionRecord({ collectionId }: { collectionId: string }): ReactNode {
  const router = useRouter();
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [matches, setMatches] = useState<ReadonlyMap<string, LibraryMatch>>(new Map());
  const [renaming, setRenaming] = useState(false);
  const [sending, setSending] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const title = load.kind === 'ready' ? load.collection.name : undefined;
  useRouteTitle(title);
  useDocumentTitle(title === undefined ? undefined : `${title} · Colecciones · ClipHub`);

  useEffect(() => {
    let active = true;
    setLoad({ kind: 'loading' });
    getLibraryCollection(collectionId)
      .then((collection) => {
        if (active) setLoad({ kind: 'ready', collection });
      })
      .catch((error: unknown) => {
        if (active) setLoad(isLibraryNotFound(error) ? { kind: 'missing' } : { kind: 'offline' });
      });
    listLibraryMatches()
      .then((list) => {
        if (active) setMatches(new Map(list.map((match) => [match.id, match])));
      })
      .catch(() => {
        // The moments still list without match names.
      });
    return () => {
      active = false;
    };
  }, [collectionId, attempt]);

  if (load.kind === 'loading') {
    return (
      <div role="status" aria-label="Cargando la colección" className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (load.kind === 'missing') {
    return (
      <EmptyState
        icon={Bookmark}
        title="Esta colección ya no existe"
        description="Puede que se haya borrado desde otra ventana."
        actions={
          <Button asChild size="sm" variant="outline">
            <Link href={COLLECTIONS_HREF}>Ver colecciones</Link>
          </Button>
        }
      />
    );
  }
  if (load.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="No se pudo abrir la colección"
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" size="sm" variant="outline" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }

  const { collection } = load;
  const missing = collection.missing_match_ids;
  const gone = new Set(missing);
  const playable = playableCount(collection.items, missing);
  const refs = collectionRefs(collection.items, missing);

  const saveItems = async (items: CollectionItem[]): Promise<boolean> => {
    setSaveError(null);
    try {
      const next = await patchLibraryCollection(collection.id, { items });
      setLoad({ kind: 'ready', collection: { ...next, missing_match_ids: missing } });
      return true;
    } catch (error) {
      setSaveError(isLibraryOffline(error) ? 'El servicio local no responde. El cambio no se guardó.' : 'No se pudo guardar el cambio.');
      return false;
    }
  };

  return (
    <div className="flex flex-col gap-5">
      <RecordHeader
        kind="Colección"
        icon={Bookmark}
        title={collection.name}
        fields={[
          { label: 'Momentos', value: collection.items.length },
          { label: 'Última edición', value: formatShortDate(collection.updated_at) },
        ]}
        actions={
          <>
            {playable === 0 ? null : (
              <Button asChild size="sm">
                <Link href={replayHref(refs, collection.name)}>
                  <Play aria-hidden />
                  Reproducir
                </Link>
              </Button>
            )}
            {playable === 0 ? null : (
              <Button type="button" size="sm" variant="outline" onClick={() => setSending(true)}>
                <Smartphone aria-hidden />
                Enviar a Short
              </Button>
            )}
            <Button type="button" size="sm" variant="outline" onClick={() => setRenaming(true)}>
              <Pencil aria-hidden />
              Renombrar
            </Button>
            <DeleteMatchButton
              label={collection.name}
              confirmText={`¿Borrar ${momentCountLabel(collection.items.length)}?`}
              onConfirm={() => deleteLibraryCollection(collection.id)}
              onDeleted={() => router.push(COLLECTIONS_HREF)}
            />
          </>
        }
      />

      {missing.length === 0 ? null : (
        <p className="flex items-start gap-2 rounded-md border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning">
          <TriangleAlert className="mt-0.5 size-4 shrink-0" aria-hidden />
          {collection.items.length - playable} de {momentCountLabel(collection.items.length)} son de partidas que ya no están en la
          biblioteca. No se reproducen ni van a un Short; quítalos o vuelve a importar esas demos.
        </p>
      )}
      {saveError === null ? null : (
        <p role="alert" className="text-body-sm text-destructive">
          {saveError}
        </p>
      )}

      {collection.items.length === 0 ? (
        <EmptyState
          icon={Bookmark}
          title="Esta colección no tiene momentos"
          description="En la repetición de una partida, pulsa Guardar momento y elige esta colección."
        />
      ) : (
        <ol className="studio-panel flex flex-col divide-y divide-border-subtle">
          {collection.items.map((item, index) => (
            <MomentRow
              key={`${item.match_id}:${item.round}:${item.tick}`}
              item={item}
              position={index + 1}
              match={matches.get(item.match_id)}
              gone={gone.has(item.match_id)}
              first={index === 0}
              last={index === collection.items.length - 1}
              onMove={(delta) => void saveItems(moveItem(collection.items, index, delta))}
              onRemove={() => void saveItems(removeItem(collection.items, index))}
              onNote={(note) => saveItems(setItemNote(collection.items, index, note))}
            />
          ))}
        </ol>
      )}

      {renaming ? (
        <RenameCollectionDialog
          collection={collection}
          onClose={() => setRenaming(false)}
          onRenamed={(next) => setLoad({ kind: 'ready', collection: { ...next, missing_match_ids: missing } })}
        />
      ) : null}
      {sending ? (
        <AnalysisVideoDialog
          title={`Enviar ${collection.name} a un Short`}
          sources={groupEvidenceByMatch(collection.items.filter((item) => !gone.has(item.match_id)))}
          onClose={() => setSending(false)}
        />
      ) : null}
    </div>
  );
}

function MomentRow({
  item,
  position,
  match,
  gone,
  first,
  last,
  onMove,
  onRemove,
  onNote,
}: {
  item: CollectionItem;
  position: number;
  match: LibraryMatch | undefined;
  gone: boolean;
  first: boolean;
  last: boolean;
  onMove: (delta: number) => void;
  onRemove: () => void;
  onNote: (note: string) => Promise<boolean>;
}): ReactNode {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(item.note);
  const [busy, setBusy] = useState(false);
  let where = 'Partida';
  if (gone) where = 'Partida borrada de la biblioteca';
  else if (match !== undefined) where = `${prettyMapName(match.map)} · ${matchTitle(match)} · ${formatShortDate(match.played_at)}`;

  const save = async (): Promise<void> => {
    setBusy(true);
    const ok = await onNote(draft);
    setBusy(false);
    if (ok) setEditing(false);
  };

  return (
    <li className={cn('flex flex-wrap items-start gap-3 px-4 py-3', gone && 'opacity-70')}>
      <span className="w-6 pt-0.5 text-right text-body-sm tabular-nums text-fg-3">{position}</span>
      <div className="flex min-w-0 flex-1 flex-col gap-1">
        <span className="text-body-sm font-medium text-fg-1">
          Ronda {item.round}
          <span className="font-normal text-fg-3"> · {where}</span>
        </span>
        {editing ? (
          <div className="flex flex-col gap-2">
            <textarea
              value={draft}
              maxLength={COLLECTION_NOTE_MAX}
              onChange={(event) => setDraft(event.target.value)}
              aria-label={`Nota del momento ${position}`}
              className={NOTE_FIELD_CLASS}
              autoFocus
            />
            <span className="flex gap-2">
              <Button type="button" size="sm" disabled={busy || draft.trim() === item.note} onClick={() => void save()}>
                {busy ? 'Guardando…' : 'Guardar nota'}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="ghost"
                disabled={busy}
                onClick={() => {
                  setDraft(item.note);
                  setEditing(false);
                }}
              >
                Cancelar
              </Button>
            </span>
          </div>
        ) : (
          <button
            type="button"
            onClick={() => setEditing(true)}
            className={cn('w-fit rounded-sm text-left text-body-sm hover:underline', item.note === '' ? 'text-fg-3' : 'text-fg-2')}
          >
            {item.note === '' ? 'Añadir nota' : item.note}
          </button>
        )}
      </div>
      <span className="flex items-center gap-1">
        {gone ? null : (
          <Button asChild size="sm" variant="ghost">
            <Link href={replayHref([{ matchId: item.match_id, round: item.round, tick: item.tick }])}>Ver</Link>
          </Button>
        )}
        <Button type="button" size="icon-sm" variant="ghost" aria-label="Subir" disabled={first} onClick={() => onMove(-1)}>
          <ArrowUp aria-hidden />
        </Button>
        <Button type="button" size="icon-sm" variant="ghost" aria-label="Bajar" disabled={last} onClick={() => onMove(1)}>
          <ArrowDown aria-hidden />
        </Button>
        <Button type="button" size="icon-sm" variant="ghost" aria-label={`Quitar el momento ${position}`} onClick={onRemove}>
          <Trash2 aria-hidden />
        </Button>
      </span>
    </li>
  );
}
