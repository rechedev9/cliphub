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
  /** Selection and settings summary; null shows `hint` instead. */
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

/** Keep the detailed brief in flow; only the compact action bar follows the viewport. */
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
    <>
      <details className="group/brief mt-4 border-t border-border-subtle py-3">
        <summary
          id={briefId}
          className="flex min-h-10 cursor-pointer list-none items-center gap-2 text-body-sm font-semibold text-fg-2 focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden"
        >
          <ChevronRight aria-hidden className="size-4 transition-transform duration-(--dur-fast) group-open/brief:rotate-90" />
          Configuración del vídeo
          <span className="text-fg-3">· {briefItems.length} ajustes</span>
        </summary>
        <section aria-labelledby={briefId} className="studio-panel mt-2 px-4 py-3">
          <CreativeBriefList items={briefItems} className="@[42rem]/content:grid-cols-2 @[70rem]/content:grid-cols-3" />
          {briefNote}
        </section>
      </details>
      <div
        data-slot="produce-actions"
        className={cn(
          'sticky bottom-0 z-20 mt-2 border-t bg-surface-1 px-1 py-3.5',
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

          {!busy && (summary !== null || ready) ? (
            <p role="status" className="text-body-sm text-fg-2">
              {nextStep}
            </p>
          ) : null}

          <div className="flex flex-wrap items-center gap-x-5 gap-y-3">
            <div className="min-w-0 basis-full @[40rem]/content:basis-auto @[40rem]/content:flex-1">
              <p className="font-mono text-meta uppercase tracking-widest text-fg-3">{eyebrow}</p>
              {summary !== null ? (
                <p className="mt-0.5 break-words text-body-sm text-fg-1">{summary}</p>
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
    </>
  );
}
