'use client';
import type { ReactNode } from 'react';
import { STREAM_VARIANTS, type StreamVariant } from '@/lib/api/streams';
import { LayoutGlyph } from '@/components/streams/layout-glyph';
import { cn } from '@/lib/utils';

const LAYOUT_LABELS: Record<StreamVariant, { title: string; detail: string }> = {
  'streamer-vertical-stack-40-60': { title: 'Cámara + juego', detail: 'Cámara arriba, partida debajo' },
  'streamer-vertical-stack': { title: 'Cámara doble', detail: 'Partida entre dos bandas de cámara' },
  'streamer-fullframe-nocam': { title: 'Solo juego', detail: 'Partida a pantalla completa' },
};
export function StreamLayoutBar({
  variant,
  disabled,
  onVariantChange,
}: {
  variant: StreamVariant;
  disabled: boolean;
  onVariantChange: (variant: StreamVariant) => void;
}): ReactNode {
  return (
    <div role="group" aria-label="Aspecto del Short" className="flex flex-col gap-2">
      {STREAM_VARIANTS.map((entry) => {
        const active = entry.value === variant;
        const label = LAYOUT_LABELS[entry.value];
        return (
          <button
            key={entry.value}
            type="button"
            disabled={disabled}
            aria-pressed={active}
            onClick={() => onVariantChange(entry.value)}
            className={cn(
              'flex items-center gap-3 rounded-md border px-3 py-2 text-left focus-visible:outline-2 focus-visible:outline-ring disabled:opacity-50',
              active ? 'border-stream bg-stream/10' : 'border-border-subtle hover:bg-surface-3',
            )}
          >
            <LayoutGlyph variant={entry.value} selected={active} />
            <span>
              <span className="block text-body-sm font-semibold">{label.title}</span>
              <span className="block text-label text-fg-3">{label.detail}</span>
            </span>
          </button>
        );
      })}
    </div>
  );
}
