'use client';

import { useState, type ReactNode } from 'react';
import { Trash2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';
import { deleteErrorMessage, deleteFeedbackTone } from '@/lib/delete-error';
import { cn } from '@/lib/utils';

export function DeleteMatchButton({
  label,
  confirmText,
  onConfirm,
  onDeleted,
  messageBelow = false,
}: {
  label: string;
  messageBelow?: boolean;
  confirmText?: string | undefined;
  onConfirm: () => Promise<void>;
  onDeleted: () => void;
}): ReactNode {
  const [error, setError] = useState<string | null>(null);

  async function confirm(): Promise<void> {
    setError(null);
    try {
      await onConfirm();
      onDeleted();
    } catch (err) {
      setError(deleteErrorMessage(err));
    }
  }

  return (
    <div className={messageBelow ? 'contents' : 'flex shrink-0 flex-col items-end gap-1'}>
      <ConfirmDialog
        title={confirmText ?? `¿Borrar ${label}?`}
        description={`Se borra ${label}. No se puede deshacer.`}
        confirmLabel="Borrar"
        pendingLabel="Borrando…"
        onConfirm={confirm}
        trigger={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={`Borrar ${label}`}
            title={`Borrar ${label}`}
            className="text-fg-3 hover:bg-destructive/10 hover:text-destructive"
          >
            <Trash2 aria-hidden />
          </Button>
        }
      />
      {error ? (
        <p
          role="status"
          className={cn(
            messageBelow ? 'basis-full text-body-sm' : 'max-w-[13rem] text-right text-meta leading-tight',
            deleteFeedbackTone(error) === 'notice' ? 'text-fg-2' : 'text-destructive',
          )}
        >
          {error}
        </p>
      ) : null}
    </div>
  );
}
