'use client';

import { useState, type ReactNode } from 'react';
import { ChevronDown } from 'lucide-react';
import { BRIEF_APPROVAL_LABEL, type CreativeBriefItem } from '@/lib/reel-brief';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * The approval gate and the one action this screen exists for. The brief is
 * summarised always and shown in full on demand; the checkbox only becomes
 * available once the plan is renderable, and the caller resets it on any
 * plan change. While something blocks approval the row names it.
 */
export function StreamFooter({
  briefLine,
  briefItems,
  briefApproved,
  briefApprovable,
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
  briefLine: string;
  briefItems: CreativeBriefItem[];
  briefApproved: boolean;
  briefApprovable: boolean;
  /** What still blocks approval; shown while `briefApprovable` is false. */
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
  const [briefOpen, setBriefOpen] = useState(false);

  return (
    <footer className="flex shrink-0 flex-col gap-2.5 border-t border-stream/45 bg-surface-1 px-(--shell-gutter) py-3 shadow-[var(--elev-3)]">
      <section className="studio-panel flex flex-col gap-2 px-3.5 py-2.5" aria-labelledby="stream-creative-brief-title">
        <div className="flex items-center gap-4">
          <span id="stream-creative-brief-title" className="shrink-0 font-mono text-meta uppercase tracking-widest text-stream-text">
            Brief creativo
          </span>
          <span className="min-w-0 flex-1 truncate font-mono text-label uppercase text-fg-2" title={briefLine}>
            {briefLine}
          </span>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-expanded={briefOpen}
            aria-label={briefOpen ? 'Ocultar el brief completo' : 'Ver el brief completo'}
            onClick={() => setBriefOpen((open) => !open)}
          >
            <ChevronDown aria-hidden className={cn('transition-transform duration-(--dur-fast)', briefOpen && 'rotate-180')} />
          </Button>
          <label
            className={cn(
              'flex min-h-10 shrink-0 items-center gap-2 font-mono text-meta uppercase tracking-wider',
              briefApproved ? 'text-success' : 'text-fg-2',
              !briefApprovable && 'text-fg-4',
            )}
          >
            <input
              type="checkbox"
              checked={briefApproved}
              disabled={busy || rendering || !briefApprovable}
              onChange={(event) => onBriefApprovedChange(event.target.checked)}
              className="size-[18px] shrink-0 cursor-pointer accent-success disabled:cursor-not-allowed"
            />
            {BRIEF_APPROVAL_LABEL}
          </label>
        </div>
        {blockerHint === null || briefApprovable ? null : (
          <p className="font-mono text-meta uppercase tracking-wider text-warning">{blockerHint}</p>
        )}
        {briefOpen ? (
          <dl className="grid gap-x-6 gap-y-1 border-t border-border-subtle pt-2 text-body-sm sm:grid-cols-2 lg:grid-cols-4">
            {briefItems.map((item) => (
              <div key={item.label} className="flex min-w-0 gap-1.5">
                <dt className="shrink-0 text-fg-3">{item.label}:</dt>
                <dd className="truncate text-fg-1" title={item.value}>
                  {item.value}
                </dd>
              </div>
            ))}
          </dl>
        ) : null}
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
