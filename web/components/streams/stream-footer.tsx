'use client';

import { useId, type ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { BriefApprovalCheckbox, CreativeBriefList } from '@/components/studio/creative-brief';
import { Button } from '@/components/ui/button';

/** Outer separator for the one-line brief; values already use ` · ` inside. */
const BRIEF_LINE_SEPARATOR = ' — ';

/**
 * The approval gate and the one action this screen exists for. The brief is
 * summarised always and shown in full on demand; the checkbox only becomes
 * available once nothing blocks the render, and the caller resets it on any
 * plan change. While something blocks approval the row names it.
 */
export function StreamFooter({
  briefItems,
  briefApproved,
  blockerHint,
  countLabel,
  summary,
  ctaLabel,
  ctaDisabled,
  rendering,
  busy,
  onBriefApprovedChange,
  onCreate,
  onBack,
}: {
  briefItems: CreativeBriefItem[];
  briefApproved: boolean;
  /** What still blocks approval; null once the brief can be approved. */
  blockerHint: string | null;
  countLabel: string;
  summary: string;
  ctaLabel: string;
  ctaDisabled: boolean;
  rendering: boolean;
  busy: boolean;
  onBriefApprovedChange: (approved: boolean) => void;
  onCreate: () => void;
  onBack: () => void;
}): ReactNode {
  const briefId = useId();
  const briefLine = briefItems.map((item) => item.value).join(BRIEF_LINE_SEPARATOR);
  const approvable = blockerHint === null;

  return (
    <footer className="flex shrink-0 flex-col gap-2.5 border-t border-stream/45 bg-surface-1 px-(--shell-gutter) py-3 shadow-[var(--elev-3)]">
      <section className="studio-panel flex flex-col gap-2 px-3.5 py-2.5" aria-labelledby={briefId}>
        <div className="flex items-center gap-4">
          <details className="group/brief min-w-0 flex-1">
            <summary
              id={briefId}
              className="flex min-h-10 cursor-pointer list-none items-center gap-2 font-mono text-meta uppercase tracking-widest text-stream-text [&::-webkit-details-marker]:hidden"
            >
              <ChevronRight aria-hidden className="size-4 shrink-0 transition-transform duration-(--dur-fast) group-open/brief:rotate-90" />
              <span className="shrink-0">Brief creativo</span>
              <span className="min-w-0 flex-1 truncate text-label tracking-normal text-fg-2" title={briefLine}>
                {briefLine}
              </span>
            </summary>
            <CreativeBriefList items={briefItems} className="mt-2 border-t border-border-subtle pt-2 sm:grid-cols-2 lg:grid-cols-4" />
          </details>
          <BriefApprovalCheckbox
            checked={briefApproved}
            disabled={busy || rendering || !approvable}
            accent="stream"
            className="shrink-0 font-mono text-meta uppercase tracking-wider"
            onChange={onBriefApprovedChange}
          />
        </div>
        {blockerHint === null ? null : (
          <p className="font-mono text-meta uppercase tracking-wider text-warning">{blockerHint}</p>
        )}
      </section>

      <div className="flex items-center gap-5">
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
      </div>
    </footer>
  );
}
