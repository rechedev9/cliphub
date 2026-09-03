'use client';

import type { ReactNode } from 'react';
import { CircleCheck } from 'lucide-react';
import { Button } from '@/components/ui/button';

/** Right column shell: the active step's title, its scrollable content, and one action pinned at the foot. */
export function StreamStepPanel({
  title,
  action,
  children,
}: {
  title: string;
  action?: ReactNode;
  children: ReactNode;
}): ReactNode {
  return (
    <aside className="flex min-h-0 flex-col bg-surface-1 shadow-[inset_1px_0_0_0_var(--border-subtle)]">
      <div className="flex min-h-0 flex-1 flex-col gap-2.5 overflow-y-auto p-4">
        <h2 className="font-mono text-meta uppercase tracking-widest text-stream-text">{title}</h2>
        {children}
      </div>
      {action ? <div className="shrink-0 border-t border-border-subtle p-4">{action}</div> : null}
    </aside>
  );
}

/** Step 01 content: the crop is edited on the monitor; this is the explanation and the confirmation. */
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
        La facecam se recorta del propio frame 16:9 y se apila sobre el gameplay en la salida 9:16. Ajusta el marco
        en el monitor hasta que contenga la cara y confírmalo: el recorte inicial es solo una guía y puede caer sobre
        el radar.
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
        {faceCropReviewed ? 'Recorte confirmado' : 'Confirmar recorte de facecam'}
      </Button>
    </>
  );
}
