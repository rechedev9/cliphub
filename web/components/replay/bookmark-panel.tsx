'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { ArrowDown, ArrowUp, Trash2 } from 'lucide-react';
import { COLLECTION_NAME_MAX, COLLECTION_NOTE_MAX } from '@/lib/api/collections';
import type { CollectionItem } from '@/lib/api/collections';
import { moveItem, removeItem } from '@/lib/replay/bookmarks';
import { collectionHref } from '@/lib/clips/routes';
import type { CollectionsStore } from '@/hooks/use-collections';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { cn } from '@/lib/utils';

const NEW_COLLECTION = 'new';

export const NOTE_FIELD_CLASS =
  'min-h-16 w-full rounded-md border border-input bg-transparent px-2.5 py-1.5 text-body-sm text-fg-1 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50';

/** "Guardar momento": the playhead's match, round and tick, with an optional note, into a collection. */
export function SaveMomentDialog({
  store,
  moment,
  momentLabel,
  onClose,
}: {
  store: CollectionsStore;
  moment: CollectionItem;
  momentLabel: string;
  onClose: () => void;
}): ReactNode {
  const existing = store.collections ?? [];
  const [target, setTarget] = useState<string>(store.active?.id ?? NEW_COLLECTION);
  const [name, setName] = useState('');
  const [note, setNote] = useState('');
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const creating = target === NEW_COLLECTION;
  const canSave = !busy && (!creating || name.trim() !== '');

  const submit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (!canSave) return;
    setBusy(true);
    setProblem(null);
    const outcome = await store.save(creating ? { name: name.trim() } : { id: target }, { ...moment, note });
    setBusy(false);
    if (outcome === 'full') setProblem('Esta colección ya tiene 200 momentos. Elige otra o crea una nueva.');
    else if (outcome === 'failed') setProblem(store.error ?? 'No se pudo guardar el momento.');
    else onClose();
  };

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Guardar momento</DialogTitle>
            <DialogDescription>{momentLabel}</DialogDescription>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Colección
            <Select value={target} onValueChange={setTarget}>
              <SelectTrigger className="h-10">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {existing.map((collection) => (
                  <SelectItem key={collection.id} value={collection.id}>
                    {collection.name}
                  </SelectItem>
                ))}
                <SelectItem value={NEW_COLLECTION}>Nueva colección</SelectItem>
              </SelectContent>
            </Select>
          </label>
          {creating ? (
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Nombre
              <Input value={name} maxLength={COLLECTION_NAME_MAX} onChange={(event) => setName(event.target.value)} placeholder="Retakes de B" autoFocus />
            </label>
          ) : null}
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Nota
            <textarea value={note} maxLength={COLLECTION_NOTE_MAX} onChange={(event) => setNote(event.target.value)} className={NOTE_FIELD_CLASS} />
          </label>
          {problem === null ? null : (
            <p role="alert" className="text-body-sm text-destructive">
              {problem}
            </p>
          )}
          <DialogFooter>
            <Button type="button" variant="outline" onClick={onClose} disabled={busy}>
              Cancelar
            </Button>
            <Button type="submit" disabled={!canSave}>
              {busy ? 'Guardando…' : 'Guardar momento'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

/** The open collection's moments, in its order: jump to one, move it, or take it out. */
export function BookmarkPanel({
  store,
  momentLabel,
  isCurrent,
  onOpen,
}: {
  store: CollectionsStore;
  /** `Mirage · R12 · 1:23.4`, how a moment reads in the list. */
  momentLabel: (item: CollectionItem) => string;
  isCurrent: (item: CollectionItem) => boolean;
  onOpen: (item: CollectionItem) => void;
}): ReactNode {
  const { collections, active } = store;
  if (collections === null) return <p className="text-body-sm text-fg-3">Cargando colecciones…</p>;
  if (active === null) {
    return <p className="text-body-sm text-fg-2">Guarda un momento para empezar una colección y volver a él cuando quieras.</p>;
  }
  const change = (items: CollectionItem[]): void => {
    void store.update(active.id, items);
  };
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Select value={active.id} onValueChange={store.setActive}>
          <SelectTrigger aria-label="Colección abierta" className="h-9 min-w-0 flex-1">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {collections.map((collection) => (
              <SelectItem key={collection.id} value={collection.id}>
                {collection.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Button asChild size="xs" variant="ghost">
          <Link href={collectionHref(active.id)}>Abrir</Link>
        </Button>
      </div>
      {store.error === null ? null : (
        <p role="alert" className="text-body-sm text-destructive">
          {store.error}
        </p>
      )}
      {active.items.length === 0 ? (
        <p className="text-body-sm text-fg-3">Esta colección no tiene momentos.</p>
      ) : (
        <ol className="flex flex-col divide-y divide-border-subtle">
          {active.items.map((item, index) => (
            <li key={`${item.match_id}:${item.round}:${item.tick}`} className="flex items-start gap-1 py-1.5">
              <button
                type="button"
                onClick={() => onOpen(item)}
                aria-current={isCurrent(item) ? 'true' : undefined}
                className={cn(
                  'flex min-w-0 flex-1 flex-col rounded-sm px-1 py-0.5 text-left outline-none hover:bg-surface-3 focus-visible:ring-[3px] focus-visible:ring-ring/50',
                  isCurrent(item) && 'bg-surface-3',
                )}
              >
                <span className="truncate text-body-sm text-fg-1">{momentLabel(item)}</span>
                {item.note === '' ? null : <span className="line-clamp-2 text-meta text-fg-2">{item.note}</span>}
              </button>
              <Button type="button" size="icon-xs" variant="ghost" aria-label="Subir" disabled={index === 0} onClick={() => change(moveItem(active.items, index, -1))}>
                <ArrowUp aria-hidden />
              </Button>
              <Button
                type="button"
                size="icon-xs"
                variant="ghost"
                aria-label="Bajar"
                disabled={index === active.items.length - 1}
                onClick={() => change(moveItem(active.items, index, 1))}
              >
                <ArrowDown aria-hidden />
              </Button>
              <Button type="button" size="icon-xs" variant="ghost" aria-label="Quitar de la colección" onClick={() => change(removeItem(active.items, index))}>
                <Trash2 aria-hidden />
              </Button>
            </li>
          ))}
        </ol>
      )}
    </div>
  );
}
