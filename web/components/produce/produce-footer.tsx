'use client';

import type { ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import Link from 'next/link';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { CreativeBriefList } from '@/components/studio/creative-brief';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

const FOOTER_SHADOW = 'shadow-[var(--elev-band-up)]';

export type ProduceFooterProps = {
  /** Cyan for Short, magenta for Full POV (REC). */
  tone: 'short' | 'full';
  eyebrow: string;
  /** Mono uppercase summary line; null shows `hint` instead. */
  summary: ReactNode | null;
  hint: string;
  briefItems: ReadonlyArray<CreativeBriefItem>;
  /** Extra brief content below the items (e.g. the FACEIT note). */
  briefNote?: ReactNode;
  ready: boolean;
  busy: boolean;
  backHref: string;
  error: string | null;
  cta: ReactNode;
};

/** In-flow configuration summary keeps expanded settings from covering the editor. */
export function ProduceFooter({
  tone,
  eyebrow,
  summary,
  hint,
  briefItems,
  briefNote,
  ready,
  busy,
  backHref,
  error,
  cta,
}: ProduceFooterProps): ReactNode {
  const briefId = `produce-brief-${tone}`;
  const nextStep = ready
    ? 'Todo preparado. Al crear, ClipHub grabará en este PC; encontrarás el resultado en Demos y vídeos.'
    : hint;
  return (
    <div
      className={cn(
        'relative mt-2 border-t bg-surface-1 py-3.5',
        FOOTER_SHADOW,
        tone === 'full' ? 'border-stream/45' : 'border-border-accent',
      )}
    >
      <div className="flex flex-col gap-3">
        {error ? (
          <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
            {error}
          </p>
        ) : null}

        <div className="flex flex-col gap-3 @[56rem]/content:flex-row @[56rem]/content:items-start">
          <details open className="group/brief min-w-0 flex-1">
            <summary
              id={briefId}
              className="flex min-h-10 cursor-pointer list-none items-center gap-2 font-mono text-meta uppercase tracking-wider text-primary [&::-webkit-details-marker]:hidden"
            >
              <ChevronRight aria-hidden className="size-4 transition-transform duration-(--dur-fast) group-open/brief:rotate-90" />
              Configuración del render
              <span className="tracking-wider text-fg-3">
                · <span className="tabular-nums">{briefItems.length}</span> decisiones
              </span>
            </summary>
            <section aria-labelledby={briefId} className="studio-panel mt-2 px-4 py-3">
              <CreativeBriefList items={briefItems} className="@[42rem]/content:grid-cols-2 @[70rem]/content:grid-cols-3" />
            </section>
          </details>
        </div>
        {briefNote}
        {!busy && (summary !== null || ready) ? (
          <p role="status" className="text-body-sm text-fg-2">
            {nextStep}
          </p>
        ) : null}

        <div className="flex flex-wrap items-center gap-x-5 gap-y-3">
          <div className="min-w-0 basis-full @[40rem]/content:basis-auto @[40rem]/content:flex-1">
            <p className="font-mono text-meta uppercase tracking-widest text-fg-3">{eyebrow}</p>
            {summary !== null ? (
              <p className="mt-0.5 break-words font-mono text-body uppercase text-fg-2">{summary}</p>
            ) : (
              <p className="mt-0.5 break-words text-body-sm text-fg-2">{hint}</p>
            )}
          </div>
          <Button variant="outline" size="lg" asChild>
            <Link href={backHref}>Volver</Link>
          </Button>
          {cta}
        </div>
      </div>
    </div>
  );
}
