'use client';

import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { MAX_PLAYLIST_AUTHOR, MAX_PLAYLIST_FOLDER, MAX_PLAYLIST_NAME } from '@/lib/api/playlists';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';

export type PlaylistDetails = { name: string; folder: string; author: string };

/**
 * Names a playlist and files it: a new one, or the edit of one. `onSubmit`
 * answers an error to show, or null once it is saved.
 */
export function PlaylistDialog({
  title,
  description,
  submitLabel,
  initial,
  folders,
  onSubmit,
  onClose,
}: {
  title: string;
  description: string;
  submitLabel: string;
  initial?: PlaylistDetails;
  /** Folders already in use, offered as the folder is typed. */
  folders: readonly string[];
  onSubmit: (details: PlaylistDetails) => Promise<string | null>;
  onClose: () => void;
}): ReactNode {
  const [name, setName] = useState(initial?.name ?? '');
  const [folder, setFolder] = useState(initial?.folder ?? '');
  const [author, setAuthor] = useState(initial?.author ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const folderList = useId();

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    const failed = await onSubmit({ name: name.trim(), folder: folder.trim(), author: author.trim() });
    setBusy(false);
    if (failed === null) onClose();
    else setError(failed);
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>{title}</DialogTitle>
            <DialogDescription>{description}</DialogDescription>
          </DialogHeader>
          <Field label="Nombre" required>
            {(control) => (
              <Input {...control} value={name} maxLength={MAX_PLAYLIST_NAME} placeholder="Ejecuciones de FUT en Mirage" autoFocus onChange={(event) => setName(event.target.value)} />
            )}
          </Field>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Carpeta" hint="Escribe una nueva o elige una que ya uses.">
              {(control) => (
                <>
                  <Input {...control} list={folderList} value={folder} maxLength={MAX_PLAYLIST_FOLDER} placeholder="Sin carpeta" onChange={(event) => setFolder(event.target.value)} />
                  <datalist id={folderList}>
                    {folders.map((option) => (
                      <option key={option} value={option} />
                    ))}
                  </datalist>
                </>
              )}
            </Field>
            <Field label="Autor">
              {(control) => (
                <Input {...control} value={author} maxLength={MAX_PLAYLIST_AUTHOR} placeholder="Quién la prepara" onChange={(event) => setAuthor(event.target.value)} />
              )}
            </Field>
          </div>
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter>
            <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
              Cancelar
            </Button>
            <Button type="submit" disabled={busy || name.trim() === ''}>
              {submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
