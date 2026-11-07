'use client';

import type { ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import Link from 'next/link';
import { PRODUCE_READY_HINT } from '@/lib/produce/copy';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { CreativeBriefList } from '@/components/studio/creative-brief';
import { Button } from '@/components/ui/button';
import type { FormatTone } from '@/components/studio/format-tone';
import { cn } from '@/lib/utils';

const FOOTER_SHADOW = 'shadow-[var(--elev-band-up)]';
/** Fills the enabled CTA button the caller passes with the flow's format colour; disabled keeps its grey. */
const CTA_FILL: Readonly<Record<FormatTone, string>> = {
  short: '[&>[data-slot=button]:not(:disabled)]:bg-format-short [&>[data-slot=button]:not(:disabled)]:text-format-short-foreground [&>[data-slot=button]:not(:disabled):hover]:bg-format-short/90',
  full: '[&>[data-slot=button]:not(:disabled)]:bg-format-full [&>[data-slot=button]:not(:disabled)]:text-format-full-foreground [&>[data-slot=button]:not(:disabled):hover]:bg-format-full/90',
  stream: '[&>[data-slot=button]:not(:disabled)]:bg-format-stream [&>[data-slot=button]:not(:disabled)]:text-format-stream-foreground [&>[data-slot=button]:not(:disabled):hover]:bg-format-stream/90',
  summary: '[&>[data-slot=button]:not(:disabled)]:bg-format-summary [&>[data-slot=button]:not(:disabled)]:text-format-summary-foreground [&>[data-slot=button]:not(:disabled):hover]:bg-format-summary/90',
};

export type ProduceFooterProps = {
  /** The format of the flow: its colour fills the CTA (Short, Demo completa, multi-POV resumen). */
  tone: FormatTone;
  eyebrow: string;
  /** Selection and settings summary; null shows `hint` instead. */
  summary: ReactNode | null;
  hint: string;
  /** Turns the blocking hint into a button that takes the user to the missing decision. */
  onHintAction?: () => void;
  briefItems: ReadonlyArray<CreativeBriefItem>;
  /** Extra brief content below the items (e.g. the FACEIT note). */
  briefNote?: ReactNode;
  /** What creating does once the form is valid; defaults to where it records and where the result lands. */
  readyHint?: ReactNode;
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
  onHintAction,
  briefItems,
  briefNote,
  readyHint = PRODUCE_READY_HINT,
  ready,
  busy,
  backHref,
  error,
  cta,
}: ProduceFooterProps): ReactNode {
  const briefId = `produce-brief-${tone}`;
  const nextStep = ready ? readyHint : hint;
  const nextStepContent: ReactNode = !ready && onHintAction ? (
    <button
      type="button"
      onClick={onHintAction}
      className="rounded-sm text-left text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
    >
      {nextStep}
    </button>
  ) : nextStep;
  return (
    <>
      <details className={cn("group/brief border-t border-border-subtle py-1", tone === 'full' ? 'mt-1' : 'mt-3 @[80rem]/content:mt-5')}>
        <summary
          id={briefId}
          className={cn('flex cursor-pointer list-none items-center gap-2 text-body-sm font-semibold text-fg-2 focus-visible:outline-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden', 'min-h-8')}
        >
          <ChevronRight aria-hidden className="size-4 transition-transform duration-(--dur-fast) group-open/brief:rotate-90" />
          {/* A read-only recap of the choices above, not another place to edit them. */}
          Resumen de la configuración
        </summary>
        <section aria-labelledby={briefId} className="studio-panel mt-2 px-4 py-3">
          <CreativeBriefList items={briefItems} className="@[42rem]/content:grid-cols-2 @[70rem]/content:grid-cols-3" />
          {briefNote}
        </section>
      </details>
      <div
        data-slot="produce-actions"
        className={cn(
          'sticky bottom-0 z-20 mt-2 border-t bg-surface-1 px-1',
          'py-2',
          FOOTER_SHADOW,
          'border-border-accent',
        )}
      >
        <div className="flex flex-col gap-3">
          {error ? (
            <p role="alert" className="border border-destructive/40 bg-destructive/10 px-4 py-3 text-body-sm text-destructive">
              {error}
            </p>
          ) : null}

          <div className="flex flex-wrap items-center gap-x-2 gap-y-3">
            <div className="min-w-0 basis-full @[40rem]/content:basis-auto @[40rem]/content:flex-1">
              <p className="text-meta text-fg-3">{eyebrow}</p>
              {summary !== null ? (
                <p className="mt-0.5 break-words text-body-sm text-fg-1">{summary}</p>
              ) : (
                <p className="mt-0.5 break-words text-body-sm text-fg-2">{hint}</p>
              )}
              {!busy && (summary !== null || ready) ? <p role="status" className={tone === 'full' || ready ? 'sr-only' : 'mt-0.5 text-body-sm text-fg-2'}>{nextStepContent}</p> : null}
            </div>
            <Button variant="outline" size="sm" asChild>
              <Link href={backHref}>Volver</Link>
            </Button>
            <div className={cn('contents', CTA_FILL[tone])}>{cta}</div>
          </div>
        </div>
      </div>
    </>
  );
}
