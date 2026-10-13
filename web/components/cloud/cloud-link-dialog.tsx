'use client';

import { useEffect, useRef, type ReactNode } from 'react';
import { Check } from 'lucide-react';
import { CloudLinkProgress, useCloudLinkFlow } from '@/components/cloud/cloud-link-flow';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog';

export type CloudLinkDialogProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
};

/** Links this PC to a ClipHub account without leaving the screen: shows the code, opens the portal and waits. */
export function CloudLinkDialog({ open, onOpenChange }: CloudLinkDialogProps): ReactNode {
  const flow = useCloudLinkFlow({ active: open, openPending: true });
  const { phase, start } = flow;
  const needsCode = phase.kind !== 'linked' && phase.kind !== 'pending';

  // Opening the dialog starts a link unless one is already waiting for the user.
  const wasOpen = useRef(false);
  useEffect(() => {
    if (open && !wasOpen.current && needsCode) start();
    wasOpen.current = open;
  }, [open, needsCode, start]);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Conecta tu cuenta de ClipHub</DialogTitle>
          <DialogDescription>
            La cuenta solo hace falta para grabar en la nube. Grabar en este PC sigue funcionando sin ella.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4">
          {phase.kind === 'linked' ? (
            <>
              <p role="status" className="flex items-center gap-2 text-body text-success">
                <Check aria-hidden className="size-5" />
                Este PC ya está conectado a {phase.name}.
              </p>
              <div className="flex justify-end">
                <Button type="button" size="sm" onClick={() => onOpenChange(false)}>
                  Listo
                </Button>
              </div>
            </>
          ) : (
            <CloudLinkProgress flow={flow} />
          )}
        </div>
      </DialogContent>
    </Dialog>
  );
}
