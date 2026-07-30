'use client';

import type { ReactNode } from 'react';
import { Sparkles } from 'lucide-react';
import { Button } from '@/components/ui/button';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { canCreateStreamShorts } from '@/lib/streams/brief';

/**
 * The one action this screen exists for.
 *
 * Plan problems are reported by the editor's own alert above this bar rather
 * than by disabling the button silently — a disabled CTA with no explanation is
 * the single most common way a local pipeline looks broken. The notch ties it
 * to the demo pipeline's FORJAR REEL: the same shape means the same kind of
 * commitment. The creative brief + checkbox is the product approval gate.
 */
export function StreamRenderBar({
  rendering,
  busy,
  briefItems,
  briefApproved,
  onBriefApprovedChange,
  onCreate,
  onStartOver,
}: {
  rendering: boolean;
  busy: boolean;
  briefItems: CreativeBriefItem[];
  briefApproved: boolean;
  onBriefApprovedChange: (approved: boolean) => void;
  onCreate: () => void;
  onStartOver: () => void;
}): ReactNode {
  const ready = canCreateStreamShorts({ briefApproved, busy });
  return (
    <div className="studio-panel flex flex-col gap-4 p-4">
      <section aria-labelledby="stream-creative-brief-title">
        <p id="stream-creative-brief-title" className="font-mono text-meta uppercase tracking-wider text-primary">
          Brief creativo exacto
        </p>
        <dl className="mt-2.5 grid gap-x-6 gap-y-1.5 text-body-sm sm:grid-cols-2">
          {briefItems.map((item) => (
            <div key={item.label} className="flex min-w-0 gap-1.5">
              <dt className="shrink-0 text-fg-3">{item.label}:</dt>
              <dd className="truncate text-fg-1" title={item.value}>{item.value}</dd>
            </div>
          ))}
        </dl>
        <label className="mt-3.5 flex min-h-10 items-center gap-2.5 text-body-sm text-fg-1">
          <input
            type="checkbox"
            checked={briefApproved}
            disabled={busy || rendering}
            onChange={(event) => onBriefApprovedChange(event.target.checked)}
            className="size-5 shrink-0 cursor-pointer accent-primary disabled:cursor-not-allowed disabled:opacity-50"
          />
          Apruebo todas estas decisiones antes de iniciar el render.
        </label>
      </section>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-3">
        <Button
          type="button"
          variant="hero"
          size="lg"
          className="neon-notch"
          onClick={onCreate}
          disabled={!ready}
          loading={busy}
        >
          {busy ? null : <Sparkles className="size-4" aria-hidden />}
          {rendering ? 'RENDERIZANDO…' : 'CREAR SHORTS'}
        </Button>

        <Button type="button" variant="ghost" onClick={onStartOver} disabled={busy} className="ml-auto">
          Empezar de nuevo
        </Button>
      </div>
    </div>
  );
}
