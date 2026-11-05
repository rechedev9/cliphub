'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { isLibraryOffline, patchLibraryCollection } from '@/lib/api/library';
import { COLLECTION_NAME_MAX } from '@/lib/api/collections';
import type { LibraryCollection } from '@/lib/api/collections';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';

/** Renames a collection; the moments stay as they are. */
export function RenameCollectionDialog({
  collection,
  onClose,
  onRenamed,
}: {
  collection: Pick<LibraryCollection, 'id' | 'name'>;
  onClose: () => void;
  onRenamed: (next: LibraryCollection) => void;
}): ReactNode {
  const [name, setName] = useState(collection.name);
  const [busy, setBusy] = useState(false);
  const [problem, setProblem] = useState<string | null>(null);
  const trimmed = name.trim();

  const submit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (trimmed === '' || busy) return;
    setBusy(true);
    setProblem(null);
    try {
      onRenamed(await patchLibraryCollection(collection.id, { name: trimmed }));
      onClose();
    } catch (error) {
      setProblem(isLibraryOffline(error) ? 'El servicio local no responde. Vuelve a intentarlo.' : 'No se pudo cambiar el nombre.');
      setBusy(false);
    }
  };

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md">
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Renombrar colección</DialogTitle>
          </DialogHeader>
          <label className="flex flex-col gap-1.5 text-body-sm text-fg-2">
            Nombre
            <Input value={name} maxLength={COLLECTION_NAME_MAX} onChange={(event) => setName(event.target.value)} autoFocus />
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
            <Button type="submit" disabled={trimmed === '' || trimmed === collection.name || busy}>
              {busy ? 'Guardando…' : 'Guardar nombre'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
