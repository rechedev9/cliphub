'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import type { LibretaStatus } from '@/lib/api/libreta';
import { patchLibraryLineup, type LibraryLineup, type LineupPatch } from '@/lib/api/lineups';
import { parseTags, tagsText } from '@/lib/libreta/status';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { TechniqueFields, type LineupTechnique } from './lineup-fields';
import { Field, TagsField, TitleField } from './play-fields';
import { StatusSelect } from './status-badge';

/** Edits a lineup's name, status, tags and technique; map, side and grenade stay what it is. */
export function EditLineupDialog({ lineup, onClose, onSaved }: { lineup: LibraryLineup; onClose: () => void; onSaved: () => void }): ReactNode {
  const [title, setTitle] = useState(lineup.title);
  const [status, setStatus] = useState<LibretaStatus>(lineup.status);
  const [tags, setTags] = useState(tagsText(lineup.tags));
  const [technique, setTechnique] = useState<LineupTechnique>({ movement: lineup.movement, jump: lineup.jump, click: lineup.click });
  // Sending the technique marks it as checked by the user, so it goes only once they touch it.
  const [techniqueTouched, setTechniqueTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    const patch: LineupPatch = { title: title.trim(), status, tags: parseTags(tags) };
    try {
      await patchLibraryLineup(lineup.id, techniqueTouched ? { ...patch, ...technique } : patch);
      onSaved();
      onClose();
    } catch {
      setError('No se pudo guardar el lineup.');
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Editar el lineup</DialogTitle>
            <DialogDescription>El mapa, el lado y la granada no cambian: con otros, es otro lineup.</DialogDescription>
          </DialogHeader>
          <TitleField value={title} onChange={setTitle} />
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Estado">
              <StatusSelect value={status} onChange={setStatus} />
            </Field>
            <TagsField value={tags} onChange={setTags} />
          </div>
          <TechniqueFields
            value={technique}
            onChange={(next) => {
              setTechnique(next);
              setTechniqueTouched(true);
            }}
          />
          {lineup.technique_estimated && !techniqueTouched ? (
            <p className="text-meta text-fg-3">Studio estimó la técnica con la demo. Cuando la cambies, queda como revisada.</p>
          ) : null}
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
      </DialogContent>
    </Dialog>
  );
}
