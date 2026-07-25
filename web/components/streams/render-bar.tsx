'use client';

import type { ReactNode } from 'react';
import { Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';

/**
 * The one action this screen exists for.
 *
 * Plan problems are reported by the editor's own alert above this bar rather
 * than by disabling the button silently — a disabled CTA with no explanation is
 * the single most common way a local pipeline looks broken. The notch ties it
 * to the demo pipeline's FORJAR REEL: the same shape means the same kind of
 * commitment.
 */
export function StreamRenderBar({
  rendering,
  busy,
  onCreate,
  onStartOver,
}: {
  rendering: boolean;
  busy: boolean;
  onCreate: () => void;
  onStartOver: () => void;
}): ReactNode {
  return (
    <div className="studio-panel flex flex-wrap items-center gap-x-4 gap-y-3 p-4">
      <Button
        type="button"
        variant="hero"
        size="lg"
        className="neon-notch"
        onClick={onCreate}
        disabled={busy}
        loading={busy}
      >
        {busy ? null : <Sparkles className="size-4" aria-hidden />}
        {rendering ? 'RENDERIZANDO…' : 'CREAR SHORTS'}
      </Button>

      <Button type="button" variant="ghost" onClick={onStartOver} disabled={busy} className="ml-auto">
        Empezar de nuevo
      </Button>
    </div>
  );
}
