'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { RotateCcw } from 'lucide-react';
import { MAX_CALLOUT_NAME, deleteCalloutName, setCalloutName, type MapCallout } from '@/lib/api/callouts';
import { isLibraryNotFound } from '@/lib/api/library';
import { prettyMapName } from '@/lib/format';
import { placeLabel } from '@/lib/tactical-labels';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Field } from './play-fields';

/** The team's own name for one CS2 place; `onSaved` gets the new name, or null back to CS2's. */
export function RenameCalloutDialog({
  map,
  callout,
  onClose,
  onSaved,
}: {
  map: string;
  callout: MapCallout;
  onClose: () => void;
  onSaved: (name: string | null) => void;
}): ReactNode {
  const cs2 = placeLabel(callout.token);
  const [name, setName] = useState(callout.name ?? '');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const trimmed = name.trim();

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const saved = await setCalloutName({ map, token: callout.token, name: trimmed });
      onSaved(saved.name);
      onClose();
    } catch {
      setError('No se pudo guardar el nombre. Comprueba que ClipHub Studio sigue abierto.');
      setBusy(false);
    }
  }

  async function reset(): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      await deleteCalloutName(map, callout.token);
    } catch (failure: unknown) {
      // A 404 means the place already had CS2's name.
      if (!isLibraryNotFound(failure)) {
        setError('No se pudo volver al nombre de CS2.');
        setBusy(false);
        return;
      }
    }
    onSaved(null);
    onClose();
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-md" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>Nombre de {cs2}</DialogTitle>
            <DialogDescription>
              Studio usará este nombre en {prettyMapName(map)} en lugar del de CS2: en los lineups, los roles, las jugadas y las granadas.
            </DialogDescription>
          </DialogHeader>
          <Field label="Nombre del equipo">
            <Input value={name} maxLength={MAX_CALLOUT_NAME} placeholder={cs2} autoFocus onChange={(event) => setName(event.target.value)} />
          </Field>
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          <DialogFooter className="sm:justify-between">
            {callout.name ? (
              <Button type="button" variant="ghost" disabled={busy} onClick={() => void reset()}>
                <RotateCcw aria-hidden />
                Volver al nombre de CS2
              </Button>
            ) : (
              <span aria-hidden />
            )}
            <div className="flex flex-col-reverse gap-2 sm:flex-row">
              <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
                Cancelar
              </Button>
              <Button type="submit" disabled={busy || trimmed === '' || trimmed === (callout.name ?? '')}>
                Guardar
              </Button>
            </div>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
