'use client';

import { useState } from 'react';
import { Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import type { Video } from '@/lib/api/types';
import { api } from '@/lib/api';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';

/**
 * Trash button with a confirm dialog: removes the reel from the library and
 * deletes its rendered files from disk (best-effort when the orchestrator is
 * offline). Deletion is not undoable, hence the explicit confirmation.
 */
export function DeleteVideoButton({ video, onDeleted }: { video: Video; onDeleted: () => void }) {
  const [open, setOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);

  async function onConfirm() {
    if (deleting) return;
    setDeleting(true);
    try {
      await api.deleteVideo(video.id);
      setOpen(false);
      toast('Reel borrado.');
      onDeleted();
    } catch {
      toast('No se pudo borrar el reel.');
    } finally {
      setDeleting(false);
    }
  }

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

      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="max-w-sm">
          <DialogHeader>
            <DialogTitle className="uppercase">¿Borrar este reel?</DialogTitle>
            <DialogDescription className="break-words">
              <span className="font-medium text-fg-1">{video.title}</span> y su archivo renderizado se
              eliminarán. Esta acción no se puede deshacer.
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              variant="outline"
              onClick={() => setOpen(false)}
              disabled={deleting}
              className="font-display font-bold tracking-wide uppercase"
            >
              Cancelar
            </Button>
            <Button
              variant="destructive"
              onClick={onConfirm}
              loading={deleting}
              className="font-display font-bold tracking-wide uppercase"
            >
              {deleting ? (
                'Borrando…'
              ) : (
                <>
                  <Trash2 className="size-4" aria-hidden /> Borrar
                </>
              )}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
