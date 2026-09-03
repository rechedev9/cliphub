'use client';

import type { ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import Link from 'next/link';
import { BRIEF_APPROVAL_LABEL, type CreativeBriefItem } from '@/lib/reel-brief';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** Same lift as the old CreateReelBar: the bar floats over the scrolled list. */
const FOOTER_SHADOW = 'shadow-[0_-12px_28px_-18px_oklch(0.02_0.02_264/0.9)]';

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
  briefApproved: boolean;
  /** The checkbox stays inert until every decision exists. */
  briefReady: boolean;
  onBriefApprovedChange: (approved: boolean) => void;
  backHref: string;
  busy: boolean;
  error: string | null;
  cta: ReactNode;
};

/**
 * Sticky produce footer. Approval must answer the shown brief, so the checkbox
 * is always visible and resets whenever the caller changes any decision.
 */
export function ProduceFooter({
  tone,
  eyebrow,
  summary,
  hint,
  briefItems,
  briefNote,
  briefApproved,
  briefReady,
  onBriefApprovedChange,
  backHref,
  busy,
  error,
  cta,
}: ProduceFooterProps): ReactNode {
  const briefId = `produce-brief-${tone}`;
  return (
    <div
      className={cn(
        'sticky bottom-0 z-20 -mx-(--shell-gutter) mt-2 border-t bg-surface-1 px-(--shell-gutter) py-3.5',
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

        <div className="flex flex-wrap items-start gap-x-6 gap-y-2">
          <details className="group/brief min-w-0 flex-1">
            <summary
              id={briefId}
              className="flex min-h-10 cursor-pointer list-none items-center gap-2 font-mono text-meta uppercase tracking-wider text-primary [&::-webkit-details-marker]:hidden"
            >
              <ChevronRight aria-hidden className="size-4 transition-transform duration-(--dur-fast) group-open/brief:rotate-90" />
              Brief creativo
              <span className="tracking-wider text-fg-3">
                · <span className="tabular-nums">{briefItems.length}</span> decisiones
              </span>
            </summary>
            <section aria-labelledby={briefId} className="studio-panel mt-2 px-4 py-3">
              <dl className="grid gap-x-6 gap-y-1.5 text-body-sm @[42rem]/content:grid-cols-2 @[70rem]/content:grid-cols-3">
                {briefItems.map((item) => (
                  <div key={item.label} className="flex min-w-0 gap-1.5">
                    <dt className="shrink-0 text-fg-3">{item.label}:</dt>
                    <dd className="truncate text-fg-1" title={item.value}>
                      {item.value}
                    </dd>
                  </div>
                ))}
              </dl>
              {briefNote}
            </section>
          </details>

          <label className="flex min-h-10 shrink-0 items-center gap-2.5 text-body-sm text-fg-1">
            <input
              type="checkbox"
              checked={briefApproved}
              disabled={!briefReady || busy}
              onChange={(event) => onBriefApprovedChange(event.target.checked)}
              className="size-5 shrink-0 cursor-pointer accent-primary disabled:cursor-not-allowed disabled:opacity-50"
            />
            {BRIEF_APPROVAL_LABEL}
          </label>
        </div>

        <div className="flex flex-wrap items-center gap-x-5 gap-y-3">
          <div className="min-w-0 flex-1">
            <p className="font-mono text-meta uppercase tracking-widest text-fg-3">{eyebrow}</p>
            {summary !== null ? (
              <p className="mt-0.5 truncate font-mono text-body uppercase text-fg-2">{summary}</p>
            ) : (
              <p className="mt-0.5 truncate text-body-sm text-fg-2">{hint}</p>
            )}
          </div>
          <Button variant="outline" size="sm" asChild>
            <Link href={backHref}>Volver</Link>
          </Button>
          {cta}
        </div>
      </div>
    </div>
  );
}
