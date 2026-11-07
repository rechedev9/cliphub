'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { BookmarkPlus } from 'lucide-react';
import { LibraryApiError } from '@/lib/api/library';
import { createLineupFromThrow, type LibraryLineup, type LineupThrow } from '@/lib/api/lineups';
import { lineupHref } from '@/lib/clips/routes';
import { MAX_LINEUP_TITLE, lineupSaveError } from '@/lib/libreta/lineups';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { TitleField } from './play-fields';

/** "Guardar lineup": disabled with the reason as its title when the throw cannot be read. */
export function SaveLineupButton({ onClick, disabledReason }: { onClick: () => void; disabledReason?: string }): ReactNode {
  return (
    <span title={disabledReason}>
      <Button type="button" variant="ghost" size="xs" disabled={disabledReason !== undefined} onClick={onClick}>
        <BookmarkPlus aria-hidden />
        Guardar lineup
      </Button>
    </span>
  );
}

/** Saves one throw of a library round as a Libreta lineup; the server reads where and how it was thrown. */
export function SaveLineupDialog({ from, label, onClose }: { from: LineupThrow; label: string; onClose: () => void }): ReactNode {
  const [title, setTitle] = useState(label.slice(0, MAX_LINEUP_TITLE));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<LibraryLineup | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      setSaved(await createLineupFromThrow({ title: title.trim(), from }));
    } catch (failure: unknown) {
      setError(lineupSaveError(failure instanceof LibraryApiError ? failure.code : undefined));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        {saved ? (
          <div className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardado en la Libreta</DialogTitle>
              <DialogDescription>{saved.title} ya está en la pestaña Lineups, listo para practicar en CS2.</DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                Cerrar
              </Button>
              <Button asChild>
                <Link href={lineupHref(saved.id)}>Abrir el lineup</Link>
              </Button>
            </DialogFooter>
          </div>
        ) : (
          <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardar lineup</DialogTitle>
              <DialogDescription>
                Studio guarda desde dónde se lanzó, adónde apuntaba y dónde cayó, para que lo practiques en CS2.
              </DialogDescription>
            </DialogHeader>
            <TitleField value={title} onChange={setTitle} />
            {error ? (
              <p role="alert" className="text-body-sm text-destructive">
                {error}
              </p>
            ) : null}
            <DialogFooter>
              <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
                Cancelar
              </Button>
              <Button type="submit" disabled={busy || title.trim() === ''}>
                Guardar
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
