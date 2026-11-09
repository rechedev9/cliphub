'use client';

import { useCallback, useEffect, useState } from 'react';
import { writeClipboardText } from '@/lib/clipboard-write';
import { withSetpos } from '@/lib/replay/setpos';
import type { SetposCopy } from '@/lib/replay/setpos';

/** The notice over the map after a copy; `id` restarts it when the same line is copied twice. */
export type SetposNote = { id: number; ok: boolean; text: string };

const NOTE_MS = 2500;

/** The setpos lines copied in this visit, newest first, and the notice each copy shows. */
export function useSetposCopies(): {
  list: readonly SetposCopy[];
  note: SetposNote | null;
  copy: (entry: Omit<SetposCopy, 'id'>) => void;
  copyAgain: (entry: SetposCopy) => void;
} {
  const [list, setList] = useState<SetposCopy[]>([]);
  const [note, setNote] = useState<SetposNote | null>(null);

  const write = useCallback((entry: Omit<SetposCopy, 'id'>) => {
    const show = (ok: boolean, text: string): void => setNote((last) => ({ id: (last?.id ?? 0) + 1, ok, text }));
    writeClipboardText(entry.command).then(
      () => show(true, `Setpos de ${entry.name} copiado`),
      () => show(false, 'No se pudo copiar: está en la lista de jugadores'),
    );
  }, []);
  const copy = useCallback(
    (entry: Omit<SetposCopy, 'id'>) => {
      setList((current) => withSetpos(current, entry));
      write(entry);
    },
    [write],
  );

  useEffect(() => {
    if (note === null) return;
    const timer = window.setTimeout(() => setNote(null), NOTE_MS);
    return () => window.clearTimeout(timer);
  }, [note]);

  return { list, note, copy, copyAgain: write };
}
