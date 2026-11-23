'use client';

import { useState, type ReactNode } from 'react';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function useDiscardGuard({ dirty, busy, onClose }: { dirty: boolean; busy: boolean; onClose: () => void }): {
  onOpenChange: (open: boolean) => void;
  confirm: ReactNode;
} {
  const [asking, setAsking] = useState(false);
  function onOpenChange(open: boolean): void {
    if (open || busy) return;
    if (dirty) setAsking(true);
    else onClose();
  }
  const confirm = (
    <ConfirmDialog
      open={asking}
      onOpenChange={setAsking}
      title="¿Descartar los cambios?"
      description="Lo que has escrito en este formulario no se guarda."
      confirmLabel="Descartar"
      cancelLabel="Seguir editando"
      onConfirm={onClose}
    />
  );
  return { onOpenChange, confirm };
}
