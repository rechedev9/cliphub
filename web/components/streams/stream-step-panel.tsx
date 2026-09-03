'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, CircleCheck } from 'lucide-react';
import type { NormalizedRect } from '@/lib/api/streams';
import { DEFAULT_FACE_CROP } from '@/lib/streams/plan';
import { Button } from '@/components/ui/button';
import { CropPicker } from '@/components/streams/crop-picker';
import { StreamStepCard } from '@/components/streams/step-card';

/** Right column shell: the active step's title over its scrollable content. */
export function StreamStepPanel({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <aside className="flex min-h-0 flex-col gap-2.5 overflow-y-auto bg-surface-1 p-4 shadow-[inset_1px_0_0_0_var(--border-subtle)]">
      <h2 className="font-mono text-meta uppercase tracking-widest text-stream-text">{title}</h2>
      {children}
    </aside>
  );
}

/** Step 01 content: the facecam crop and its explicit confirmation. */
export function StreamLayoutStep({
  needsFaceCrop,
  faceCrop,
  faceCropReviewed,
  busy,
  onFaceCropChange,
  onConfirmFaceCrop,
}: {
  needsFaceCrop: boolean;
  faceCrop?: NormalizedRect;
  faceCropReviewed: boolean;
  busy: boolean;
  onFaceCropChange: (rect: NormalizedRect) => void;
  onConfirmFaceCrop: () => void;
}): ReactNode {
  if (!needsFaceCrop) {
    return <p className="text-body-sm text-fg-2">Gameplay a pantalla completa, sin recorte de facecam.</p>;
  }
  return (
    <>
      <p className="text-body-sm text-fg-2">
        La facecam es un recorte del propio frame de la fuente (16:9); se apila sobre el gameplay en la
        salida 9:16. Confirma el recorte: nunca asumimos que contiene una cara.
      </p>
      <StreamStepCard title="Recorte de facecam · fuente 16:9">
        <CropPicker rect={faceCrop ?? DEFAULT_FACE_CROP} onChange={onFaceCropChange} disabled={busy} />
        <p className="font-mono text-meta uppercase tracking-wider text-fg-3">
          Arrastra el marco o usa las flechas · esquina para redimensionar
        </p>
      </StreamStepCard>
      <Button
        type="button"
        size="sm"
        variant={faceCropReviewed ? 'outline' : 'stream'}
        disabled={busy}
        onClick={onConfirmFaceCrop}
        className={`font-display uppercase tracking-wide ${faceCropReviewed ? 'border-success/45 text-success' : ''}`}
      >
        <CircleCheck aria-hidden />
        {faceCropReviewed ? 'Recorte confirmado' : 'Confirmar recorte de facecam'}
      </Button>
      {faceCropReviewed ? null : (
        <p role="alert" className="flex items-start gap-2 border border-warning/45 bg-warning/10 px-3 py-2 text-body-sm text-warning">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          Verifica que el marco contiene una cara: el recorte inicial es una guía y podría coincidir con el radar.
        </p>
      )}
    </>
  );
}
