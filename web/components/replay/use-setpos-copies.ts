'use client';

import { useCallback, useEffect, useState } from 'react';
import { writeClipboardText } from '@/lib/clipboard-write';
import type { SetposCopy } from '@/lib/replay/setpos';

/** The notice over the map after a copy; `id` restarts it when the same line is copied twice. */
export type SetposNote = { id: number; ok: boolean; text: string; command: string | null };

const NOTE_MS = 2500;

/** The setpos copied with a click on a player, with S or with a right-click, and the notice each copy shows. */
export function useSetposCopies(): { note: SetposNote | null; copy: (entry: Omit<SetposCopy, 'id'>) => void; unavailable: (text: string) => void } {
  const [note, setNote] = useState<SetposNote | null>(null);

  const show = useCallback((ok: boolean, text: string, command: string | null): void => setNote((last) => ({ id: (last?.id ?? 0) + 1, ok, text, command })), []);

  const copy = useCallback(
    (entry: Omit<SetposCopy, 'id'>) => {
      writeClipboardText(entry.command).then(
        () => show(true, entry.notice ?? `Setpos de ${entry.name} copiado`, entry.command),
        () => show(false, 'No se pudo copiar el setpos', entry.command),
      );
    },
    [show],
  );

  // A click that has no position to copy says so instead of doing nothing.
  const unavailable = useCallback((text: string) => show(false, text, null), [show]);

  useEffect(() => {
    if (note === null) return;
    const timer = window.setTimeout(() => setNote(null), NOTE_MS);
    return () => window.clearTimeout(timer);
  }, [note]);

  return { note, copy, unavailable };
}
