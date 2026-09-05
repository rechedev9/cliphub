'use client';

import type { ReactNode } from 'react';
import { CircleCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';

/** Right column shell: the active step's title over its scrollable content. */
export function StreamStepPanel({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <aside className="flex min-h-0 min-w-0 flex-col gap-4 overflow-x-hidden bg-surface-1 p-4 shadow-[inset_1px_0_0_0_var(--border-subtle)]">
      <h2 className="font-mono text-meta uppercase tracking-widest text-fg-3">{title}</h2>
      {children}
    </aside>
  );
}

/** Aspect step: the crop is edited on the monitor; this is the explanation and the confirmation. */
export function StreamLayoutStep({
  needsFaceCrop,
  faceCropReviewed,
  busy,
  onConfirmFaceCrop,
}: {
  needsFaceCrop: boolean;
  faceCropReviewed: boolean;
  busy: boolean;
  onConfirmFaceCrop: () => void;
}): ReactNode {
  if (!needsFaceCrop) {
    return <p className="text-body-sm text-fg-2">Gameplay a pantalla completa, sin recorte de facecam.</p>;
  }
  return (
    <>
      <p className="text-body-sm text-fg-2">
        Ajusta el marco sobre la cámara del streamer. La vista del Short muestra cómo quedará el vídeo.
      </p>
      <Button
        type="button"
        size="sm"
        variant={faceCropReviewed ? 'outline' : 'stream'}
        disabled={busy}
        onClick={onConfirmFaceCrop}
        className={`self-start font-display uppercase tracking-wide ${faceCropReviewed ? 'border-success/45 text-success' : ''}`}
      >
        <CircleCheck aria-hidden />
        {faceCropReviewed ? 'Cámara confirmada · continuar' : 'Confirmar cámara y continuar'}
      </Button>
    </>
  );
}
