'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import Link from 'next/link';
import { createLibraryPlay } from '@/lib/api/library';
import type { LibraryPlay, PlayInput } from '@/lib/api/stratbook';
import { playHref } from '@/lib/clips/routes';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';

const MAX_TITLE = 120;

/** "Guardar como jugada": one grenade of the finder into the Libreta, with what follows it in the notes. */
export function SaveGrenadePlayDialog({
  defaultTitle,
  play,
  onClose,
}: {
  defaultTitle: string;
  play: (title: string) => PlayInput;
  onClose: () => void;
}): ReactNode {
  const [title, setTitle] = useState(defaultTitle.slice(0, MAX_TITLE));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [saved, setSaved] = useState<LibraryPlay | null>(null);
  const preview = play(title);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      setSaved(await createLibraryPlay(play(title)));
    } catch {
      setError('No se pudo guardar la jugada. Comprueba que ClipHub Studio sigue abierto.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        {saved ? (
          <>
            <DialogHeader>
              <DialogTitle>Jugada guardada</DialogTitle>
              <DialogDescription>
                {saved.title} ya está en la Libreta con {saved.examples.length} {saved.examples.length === 1 ? 'ronda' : 'rondas'} de ejemplo.
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={onClose}>
                Seguir aquí
              </Button>
              <Button asChild>
                <Link href={playHref(saved.id)}>Abrir la jugada</Link>
              </Button>
            </DialogFooter>
          </>
        ) : (
          <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
            <DialogHeader>
              <DialogTitle>Guardar como jugada</DialogTitle>
              <DialogDescription>{preview.notes}</DialogDescription>
            </DialogHeader>
            <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
              Título
              <Input value={title} maxLength={MAX_TITLE} onChange={(event) => setTitle(event.target.value)} />
            </label>
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
                Guardar jugada
              </Button>
            </DialogFooter>
          </form>
        )}
      </DialogContent>
    </Dialog>
  );
}
