'use client';

import { useCallback, useEffect, useState } from 'react';
import { createLibraryCollection, isLibraryOffline, listLibraryCollections, patchLibraryCollection } from '@/lib/api/library';
import type { CollectionItem, LibraryCollection } from '@/lib/api/collections';
import { addMoment } from '@/lib/replay/bookmarks';
import type { AddOutcome } from '@/lib/replay/bookmarks';

const ACTIVE_KEY = 'cliphub.replay.collection';

function readActive(): string | null {
  try {
    return window.localStorage.getItem(ACTIVE_KEY);
  } catch {
    return null;
  }
}

function writeActive(id: string): void {
  try {
    window.localStorage.setItem(ACTIVE_KEY, id);
  } catch {
    // A blocked storage only loses the remembered choice.
  }
}

export type SaveTarget = { id: string } | { name: string };

export type CollectionsStore = {
  collections: LibraryCollection[] | null;
  error: string | null;
  active: LibraryCollection | null;
  setActive: (id: string) => void;
  save: (target: SaveTarget, item: CollectionItem) => Promise<AddOutcome | 'failed'>;
  update: (id: string, items: CollectionItem[]) => Promise<boolean>;
};

function failure(error: unknown): string {
  return isLibraryOffline(error)
    ? 'El servicio local no responde. Comprueba que ClipHub Studio sigue abierto.'
    : 'No se pudieron guardar los cambios de la colección.';
}

/** The collections the replay saves moments into, with the one last used kept as the open one. */
export function useCollections(): CollectionsStore {
  const [collections, setCollections] = useState<LibraryCollection[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [activeId, setActiveId] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    setActiveId(readActive());
    listLibraryCollections()
      .then((list) => {
        if (alive) setCollections(list);
      })
      .catch((reason: unknown) => {
        if (!alive) return;
        setCollections([]);
        setError(isLibraryOffline(reason) ? 'El servicio local no responde.' : 'No se pudieron leer las colecciones.');
      });
    return () => {
      alive = false;
    };
  }, []);

  const replace = useCallback((next: LibraryCollection) => {
    setCollections((current) => [next, ...(current ?? []).filter((entry) => entry.id !== next.id)]);
  }, []);

  const setActive = useCallback((id: string) => {
    setActiveId(id);
    writeActive(id);
  }, []);

  const save = useCallback(
    async (target: SaveTarget, item: CollectionItem): Promise<AddOutcome | 'failed'> => {
      setError(null);
      try {
        if ('name' in target) {
          const created = await createLibraryCollection({ name: target.name, items: [item] });
          replace(created);
          setActive(created.id);
          return 'added';
        }
        const existing = collections?.find((entry) => entry.id === target.id);
        if (existing === undefined) return 'failed';
        const result = addMoment(existing.items, item);
        if (result.outcome === 'full') return 'full';
        replace(await patchLibraryCollection(target.id, { items: result.items }));
        setActive(target.id);
        return result.outcome;
      } catch (reason) {
        setError(failure(reason));
        return 'failed';
      }
    },
    [collections, replace, setActive],
  );

  const update = useCallback(
    async (id: string, items: CollectionItem[]): Promise<boolean> => {
      setError(null);
      try {
        replace(await patchLibraryCollection(id, { items }));
        return true;
      } catch (reason) {
        setError(failure(reason));
        return false;
      }
    },
    [replace],
  );

  const active = collections?.find((entry) => entry.id === activeId) ?? collections?.[0] ?? null;
  return { collections, error, active, setActive, save, update };
}
