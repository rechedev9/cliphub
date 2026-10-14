'use client';

import type { ReactNode } from 'react';
import { CircleCheck } from 'lucide-react';

export function StreamStepPanel({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <aside className="flex min-w-0 flex-col gap-4 overflow-x-hidden p-4">
      <h2 className="font-display text-body font-semibold text-fg-1">{title}</h2>
      {children}
    </aside>
  );
}

export function StreamLayoutStep({
  needsFaceCrop,
  faceCropReviewed,
}: {
  needsFaceCrop: boolean;
  faceCropReviewed: boolean;
}): ReactNode {
  if (!needsFaceCrop) {
    return <p className="text-body-sm text-fg-2">Gameplay a pantalla completa, sin cámara.</p>;
  }
  return (
    <>
      <p className="text-body-sm text-fg-2">
        Ajusta el marco sobre la cámara del streamer. La vista del Short muestra cómo quedará el vídeo.
      </p>
      {faceCropReviewed ? (
        <p role="status" className="flex items-center gap-2 text-body-sm text-success">
          <CircleCheck aria-hidden className="size-4" />
          Cámara confirmada
        </p>
      ) : (
        <p role="status" className="text-body-sm text-fg-3">
          Cuando el marco encaje, confirma la cámara con el botón de abajo.
        </p>
      )}
    </>
  );
}
