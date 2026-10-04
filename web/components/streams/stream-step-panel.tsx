'use client';

import type { ReactNode } from 'react';
import { CircleCheck } from 'lucide-react';

/**
 * Right column content: the active step's title over its content. The surface
 * and divider live on the scrolling column in StreamEditor so they run the
 * full column height however short the step is.
 */
export function StreamStepPanel({ title, children }: { title: string; children: ReactNode }): ReactNode {
  return (
    <aside className="flex min-w-0 flex-col gap-4 overflow-x-hidden p-4">
      <h2 className="font-display text-body font-semibold text-fg-1">{title}</h2>
      {children}
    </aside>
  );
}

/** Aspect step: the crop is edited on the monitor and confirmed with the footer action; this explains it. */
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
