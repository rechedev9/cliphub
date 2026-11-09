'use client';

import { useState, type ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { countOf, type CountNoun } from '@/lib/records/selection';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';

const PROGRESSIVE = { Borrar: 'Borrando', Eliminar: 'Eliminando' } as const;

type DeleteChosenButtonProps = {
  count: number;
  noun: CountNoun;
  /** The list's own word for deleting. */
  verb?: keyof typeof PROGRESSIVE;
  /** What goes and what stays, ending with whether it can be undone. */
  description: string;
  disabled?: boolean;
  /** The delete is running: the button keeps its place and says so. */
  loading?: boolean;
  onConfirm: () => void;
};

/** The red delete for the rows chosen in a list, after one confirmation that counts them. */
export function DeleteChosenButton({ count, noun, verb = 'Borrar', description, disabled, loading = false, onConfirm }: DeleteChosenButtonProps): ReactNode {
  const [confirming, setConfirming] = useState(false);
  const what = countOf(count, noun);
  return (
    <>
      <Button
        type="button"
        variant="destructive"
        size="xs"
        disabled={disabled}
        loading={loading}
        loadingText={`${PROGRESSIVE[verb]}…`}
        onClick={() => setConfirming(true)}
      >
        <Trash2 aria-hidden />
        {verb}
      </Button>
      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              ¿{verb} {what}?
            </DialogTitle>
            <DialogDescription>{description}</DialogDescription>
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
                onConfirm();
              }}
            >
              <Trash2 aria-hidden />
              {verb} {what}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
