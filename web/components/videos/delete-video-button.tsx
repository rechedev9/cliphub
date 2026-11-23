'use client';

import { useState } from 'react';
import { Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import type { Video } from '@/lib/api/types';
import { api } from '@/lib/api';
import { deleteErrorMessage } from '@/lib/delete-error';
import { Button } from '@/components/ui/button';
import { ConfirmDialog } from '@/components/ui/confirm-dialog';

export function DeleteVideoButton({ video, onDeleted }: { video: Video; onDeleted: () => void }) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Borrar ${video.title}`}
        className="text-fg-3 hover:bg-destructive/12 hover:text-destructive"
        onClick={() => setOpen(true)}
      >
        <Trash2 className="size-4" aria-hidden />
      </Button>
      <DeleteVideoDialog
        title={video.title}
        onConfirm={() => api.deleteVideo(video.id)}
        open={open}
        onOpenChange={setOpen}
        onDeleted={onDeleted}
      />
    </>
  );
}

export type DeleteVideoDialogProps = {
  title: string;
  onConfirm: () => Promise<void>;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDeleted: () => void;
  onCloseAutoFocus?: (event: Event) => void;
};

/** The confirmation alone, for a delete that starts from a menu item. */
export function DeleteVideoDialog({ title, onConfirm: remove, open, onOpenChange, onDeleted, onCloseAutoFocus }: DeleteVideoDialogProps) {
  async function onConfirm(): Promise<void> {
    try {
      await remove();
      toast('Vídeo borrado.');
      onDeleted();
    } catch (err) {
      toast.error(deleteErrorMessage(err));
    }
  }

  return (
    <ConfirmDialog
      open={open}
      onOpenChange={onOpenChange}
      onCloseAutoFocus={onCloseAutoFocus}
      title="¿Borrar este vídeo?"
      description={<span className="break-words"><span className="font-medium text-fg-1">{title}</span> y su MP4 se borrarán de este PC. No se puede deshacer.</span>}
      confirmLabel="Borrar vídeo"
      pendingLabel="Borrando…"
      onConfirm={onConfirm}
    />
  );
}
