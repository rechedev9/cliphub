'use client';

import { useState, type ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { countOf, type CountNoun } from '@/lib/records/selection';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';

type DeleteChosenButtonProps = {
  count: number;
  noun: CountNoun;
  description: string;
  disabled?: boolean;
  loading?: boolean;
  onConfirm: () => void;
};

export function DeleteChosenButton({ count, noun, description, disabled, loading = false, onConfirm }: DeleteChosenButtonProps): ReactNode {
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
        loadingText="Borrando…"
        onClick={() => setConfirming(true)}
      >
        <Trash2 aria-hidden />
        Borrar
      </Button>
      <Dialog open={confirming} onOpenChange={setConfirming}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              ¿Borrar {what}?
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
              Borrar {what}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
