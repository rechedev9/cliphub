'use client';

import { useState, type ReactNode } from 'react';
import { Trash2, X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';

type SelectionBarProps = {
  count: number;
  /** Plural noun for the dialog title, such as "partidas". */
  noun: string;
  busy: boolean;
  /** Extra text after the count, such as the size the selection frees. */
  detail?: string;
  confirmDescription: string;
  onDelete: () => void;
  onClear: () => void;
};

/** Actions for the rows picked in a list: delete them after one confirmation, or drop the selection. */
export function SelectionBar({ count, noun, busy, detail, confirmDescription, onDelete, onClear }: SelectionBarProps): ReactNode {
  const [confirming, setConfirming] = useState(false);
  return (
    <div role="region" aria-label="Acciones de la selección" className="studio-panel flex flex-wrap items-center gap-2 px-3 py-2">
      <span className="text-body-sm font-medium text-fg-1">
        {count} {count === 1 ? 'elegido' : 'elegidos'}
        {detail ? <span className="font-normal text-fg-2">{` · ${detail}`}</span> : null}
      </span>
      <Button type="button" variant="outline" size="xs" disabled={busy} onClick={() => setConfirming(true)}>
        <Trash2 aria-hidden />
        Borrar
      </Button>
      <Button type="button" variant="ghost" size="xs" className="ml-auto" disabled={busy} onClick={onClear}>
        <X aria-hidden />
        Quitar la selección
      </Button>
      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              ¿Borrar {count} {noun}?
            </DialogTitle>
            <DialogDescription>{confirmDescription} No se puede deshacer.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button type="button" variant="outline" onClick={() => setConfirming(false)}>
              Cancelar
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={() => {
                setConfirming(false);
                onDelete();
              }}
            >
              Borrar
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
