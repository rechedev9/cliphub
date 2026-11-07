'use client';

import { useState, type FormEvent, type ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import {
  CALL_SIDES,
  MAX_CALL_MEANING,
  MAX_CALL_NAME,
  createLibraryCall,
  deleteLibraryCall,
  isCallSide,
  patchLibraryCall,
  type CallSide,
  type LibraryCall,
} from '@/lib/api/calls';
import type { LibretaStatus } from '@/lib/api/libreta';
import { CALL_SIDE_LABEL } from '@/lib/libreta/calls';
import { parseTags, tagsText } from '@/lib/libreta/status';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Field, TagsField } from './play-fields';
import { StatusSelect } from './status-badge';

const TEXTAREA =
  'rounded-md border border-input bg-transparent px-3 py-2 text-body-sm text-fg-1 outline-none focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50';

/** Writes a new call, or edits one and can delete it after asking. */
export function CallDialog({ call, onClose, onSaved }: { call?: LibraryCall; onClose: () => void; onSaved: () => void }): ReactNode {
  const [name, setName] = useState(call?.name ?? '');
  const [side, setSide] = useState<CallSide>(call?.side ?? 'both');
  const [meaning, setMeaning] = useState(call?.meaning ?? '');
  const [status, setStatus] = useState<LibretaStatus>(call?.status ?? 'active');
  const [tags, setTags] = useState(tagsText(call?.tags ?? []));
  const [busy, setBusy] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>): Promise<void> {
    event.preventDefault();
    setBusy(true);
    setError(null);
    const input = { name: name.trim(), side, meaning: meaning.trim(), status, tags: parseTags(tags) };
    try {
      if (call) await patchLibraryCall(call.id, input);
      else await createLibraryCall(input);
      onSaved();
      onClose();
    } catch {
      setError(call ? 'No se pudo guardar la llamada.' : 'No se pudo crear la llamada.');
      setBusy(false);
    }
  }

  async function remove(target: LibraryCall): Promise<void> {
    setBusy(true);
    setError(null);
    try {
      await deleteLibraryCall(target.id);
      onSaved();
      onClose();
    } catch {
      setError('No se pudo eliminar la llamada.');
      setConfirming(false);
      setBusy(false);
    }
  }

  return (
    <Dialog open onOpenChange={(open) => !open && !busy && onClose()}>
      <DialogContent className="sm:max-w-lg" showCloseButton={!busy}>
        <form onSubmit={(event) => void submit(event)} className="flex flex-col gap-4">
          <DialogHeader>
            <DialogTitle>{call ? 'Editar la llamada' : 'Nueva llamada'}</DialogTitle>
            <DialogDescription>Lo que se dice en la partida y qué tiene que hacer el equipo al oírlo.</DialogDescription>
          </DialogHeader>
          <div className="grid gap-3 sm:grid-cols-[1fr_9rem]">
            <Field label="Llamada">
              <Input value={name} maxLength={MAX_CALL_NAME} placeholder="Rata, Stack B, Todos a A" onChange={(event) => setName(event.target.value)} />
            </Field>
            <Field label="Lado">
              <Select value={side} onValueChange={(next) => isCallSide(next) && setSide(next)}>
                <SelectTrigger aria-label="Lado" className="h-9 w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {CALL_SIDES.map((value) => (
                    <SelectItem key={value} value={value}>
                      {CALL_SIDE_LABEL[value]}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </Field>
          </div>
          <Field label="Qué significa">
            <textarea
              value={meaning}
              maxLength={MAX_CALL_MEANING}
              rows={4}
              placeholder="Por ejemplo: uno se queda atrás en medio y entra solo cuando caiga el primero."
              onChange={(event) => setMeaning(event.target.value)}
              className={TEXTAREA}
            />
          </Field>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Estado">
              <StatusSelect value={status} onChange={setStatus} />
            </Field>
            <TagsField value={tags} onChange={setTags} />
          </div>
          {error ? (
            <p role="alert" className="text-body-sm text-destructive">
              {error}
            </p>
          ) : null}
          {call && confirming ? (
            <div role="alertdialog" aria-label="Eliminar la llamada" className="flex flex-col gap-2 rounded-md border border-destructive/40 p-3">
              <p className="text-body-sm text-fg-1">¿Eliminar «{call.name}»? No se puede deshacer. Si solo quieres sacarla de la lista, pásala a Archivo.</p>
              <div className="flex justify-end gap-2">
                <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => setConfirming(false)}>
                  Cancelar
                </Button>
                <Button type="button" variant="destructive" size="sm" disabled={busy} onClick={() => void remove(call)}>
                  Eliminar
                </Button>
              </div>
            </div>
          ) : (
            <DialogFooter className="sm:justify-between">
              {call ? (
                <Button type="button" variant="ghost" disabled={busy} onClick={() => setConfirming(true)}>
                  <Trash2 aria-hidden />
                  Eliminar
                </Button>
              ) : (
                <span aria-hidden />
              )}
              <div className="flex flex-col-reverse gap-2 sm:flex-row">
                <Button type="button" variant="outline" disabled={busy} onClick={onClose}>
                  Cancelar
                </Button>
                <Button type="submit" disabled={busy || name.trim() === ''}>
                  {call ? 'Guardar' : 'Crear llamada'}
                </Button>
              </div>
            </DialogFooter>
          )}
        </form>
      </DialogContent>
    </Dialog>
  );
}
