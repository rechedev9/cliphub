'use client';

import type { ReactNode } from 'react';
import { Maximize2, Minus, Plus } from 'lucide-react';
import { MAX_ZOOM } from '@/lib/replay/map-view';
import { cn } from '@/lib/utils';

/** One press of a zoom button, as a multiple of the current zoom. */
export const BUTTON_STEP = 1.5;

function MapButton({ label, disabled, onClick, children }: { label: string; disabled: boolean; onClick: () => void; children: ReactNode }): ReactNode {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className="grid size-8 place-items-center rounded-sm text-fg-2 outline-none hover:bg-surface-3 hover:text-fg-1 focus-visible:ring-2 focus-visible:ring-ring disabled:text-fg-4 motion-safe:active:scale-[0.97] [&_svg]:size-4"
    >
      {children}
    </button>
  );
}

/** Zoom out, the zoom level, zoom in and the whole map: the replay's map buttons, shared by every 2D map. */
export function MapZoomControls({
  zoom,
  zoomed,
  onZoomBy,
  onReset,
  className,
}: {
  zoom: number;
  zoomed: boolean;
  onZoomBy: (factor: number) => void;
  onReset: () => void;
  className?: string;
}): ReactNode {
  return (
    // Presses stay on the buttons: the map under them must not pan, pick a point or reset.
    <div
      className={cn('flex items-center gap-0.5 rounded-md border border-border-strong bg-surface-1/95 p-0.5 print:hidden', className)}
      onPointerDown={(event) => event.stopPropagation()}
      onClick={(event) => event.stopPropagation()}
      onDoubleClick={(event) => event.stopPropagation()}
    >
      <MapButton label="Alejar" disabled={zoom <= 1} onClick={() => onZoomBy(1 / BUTTON_STEP)}>
        <Minus aria-hidden />
      </MapButton>
      <span className="w-10 text-center text-meta tabular-nums text-fg-2" aria-live="polite">
        {Math.round(zoom * 100)}%
      </span>
      <MapButton label="Acercar" disabled={zoom >= MAX_ZOOM} onClick={() => onZoomBy(BUTTON_STEP)}>
        <Plus aria-hidden />
      </MapButton>
      <MapButton label="Ver el mapa entero" disabled={!zoomed} onClick={onReset}>
        <Maximize2 aria-hidden />
      </MapButton>
    </div>
  );
}
