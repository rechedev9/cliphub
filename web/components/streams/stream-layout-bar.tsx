'use client';

import type { ReactNode } from 'react';
import { STREAM_VARIANTS, type StreamVariant } from '@/lib/api/streams';
import { LayoutGlyph } from '@/components/streams/layout-glyph';
import { cn } from '@/lib/utils';

/** 48px strip under the shell: output spec on the left, the layout segmented control on the right. */
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
    <div className="flex min-h-12 shrink-0 flex-wrap items-center gap-3 py-2 border-b border-border-subtle px-(--shell-gutter)">
      <span className="min-w-0 basis-full font-mono text-meta uppercase tracking-wider text-fg-3 @[48rem]/content:basis-auto">
        Salida 1080×1920 · un Short por corte
      </span>
      <span className="font-mono text-meta uppercase tracking-wider text-fg-3">Encuadre</span>
      <div role="group" aria-label="Encuadre" className="flex min-w-0 flex-wrap">
        {STREAM_VARIANTS.map((entry) => {
          const active = entry.value === variant;
          return (
            <button
              key={entry.value}
              type="button"
              disabled={disabled}
              aria-pressed={active}
              title={entry.subtitle}
              onClick={() => onVariantChange(entry.value)}
              className={cn(
                'inline-flex h-10 items-center gap-2 border px-3 font-mono text-meta uppercase tracking-wider transition-colors duration-(--dur-fast) ease-standard -ml-px first:ml-0',
                'focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-50',
                active
                  ? 'z-[1] border-stream bg-stream text-stream-foreground'
                  : 'border-border-strong bg-surface-2 text-fg-2 hover:border-stream/60 hover:text-fg-1',
              )}
            >
              <LayoutGlyph variant={entry.value} selected={active} size="sm" />
              {entry.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
