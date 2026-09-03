'use client';

import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';

/**
 * The one action this screen exists for. The brief itself lives in step 05
 * (Revisar y renderizar); the footer only summarises the output and moves the
 * user to whatever still blocks the render, or renders.
 */
export function StreamFooter({
  countLabel,
  summary,
  ctaLabel,
  ctaDisabled,
  rendering,
  busy,
  onCreate,
  onBack,
}: {
  countLabel: string;
  summary: string;
  ctaLabel: string;
  ctaDisabled: boolean;
  rendering: boolean;
  busy: boolean;
  onCreate: () => void;
  onBack: () => void;
}): ReactNode {
  return (
    <footer className="flex shrink-0 items-center gap-5 border-t border-stream/45 bg-surface-1 px-(--shell-gutter) py-3 shadow-[var(--elev-3)]">
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="font-mono text-meta uppercase tracking-widest text-fg-3">{countLabel}</span>
        <span className="truncate font-mono text-body uppercase text-fg-2" title={summary}>
          {summary}
        </span>
      </div>
      <Button type="button" variant="outline" size="sm" onClick={onBack} disabled={busy} className="font-display uppercase tracking-wide">
        Volver
      </Button>
      <Button
        type="button"
        variant="stream"
        size="lg"
        className="neon-notch font-display uppercase tracking-wide"
        onClick={onCreate}
        disabled={ctaDisabled}
      >
        {rendering || busy ? <span aria-hidden className="studio-spinner" /> : null}
        {ctaLabel}
      </Button>
    </footer>
  );
}
