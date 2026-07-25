'use client';

import type { ReactNode } from 'react';
import { AlertTriangle, Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';

/**
 * The one action this screen exists for, plus the reasons it is unavailable.
 *
 * A blocked render always states WHY next to the button — a disabled CTA with
 * no explanation is the single most common way a local pipeline looks broken.
 * The notch ties it to the demo pipeline's FORJAR REEL: the same shape means
 * the same kind of commitment.
 */
export function StreamRenderBar({
  rendering,
  busy,
  captionReviewBlocked,
  killfeedBlockedReason,
  onCreate,
  onStartOver,
}: {
  rendering: boolean;
  busy: boolean;
  captionReviewBlocked: boolean;
  /** Non-null when the killfeed analysis blocks the render, with the reason. */
  killfeedBlockedReason: string | null;
  onCreate: () => void;
  onStartOver: () => void;
}): ReactNode {
  const blocked = captionReviewBlocked || killfeedBlockedReason !== null;

  return (
    <div className="studio-panel flex flex-wrap items-center gap-x-4 gap-y-3 p-4">
      <Button
        type="button"
        variant="hero"
        size="lg"
        className="neon-notch"
        onClick={onCreate}
        disabled={busy || blocked}
        loading={busy}
      >
        {busy ? null : <Sparkles className="size-4" aria-hidden />}
        {rendering ? 'RENDERIZANDO…' : 'CREAR SHORTS'}
      </Button>

      {captionReviewBlocked ? (
        <p className="flex items-center gap-2 text-body-sm text-warning">
          <AlertTriangle aria-hidden className="size-4 shrink-0" />
          Revisa los subtítulos pendientes para continuar.
        </p>
      ) : null}

      {killfeedBlockedReason !== null ? (
        <p className="flex min-w-56 flex-1 items-start gap-2 text-body-sm text-warning">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {killfeedBlockedReason}
        </p>
      ) : null}

      <Button type="button" variant="ghost" onClick={onStartOver} disabled={busy} className="ml-auto">
        Empezar de nuevo
      </Button>
    </div>
  );
}
