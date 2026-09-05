'use client';
import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
export function StreamFooter({
  countLabel,
  summary,
  ctaLabel,
  ctaDisabled,
  rendering,
  onCreate,
  onBack,
  backLabel,
  action,
}: {
  countLabel: string;
  summary: string;
  ctaLabel: string;
  ctaDisabled: boolean;
  rendering: boolean;
  onCreate: () => void;
  onBack: () => void;
  backLabel: string;
  action?: ReactNode;
}): ReactNode {
  return (
    <footer className="sticky bottom-0 z-10 flex shrink-0 flex-wrap items-center gap-3 border-t border-border bg-surface-1 px-4 py-3">
      <div className="min-w-0 flex-1">
        <p className="font-semibold text-body">{countLabel}</p>
        <p className="text-label text-fg-3">{summary}</p>
      </div>
      <Button type="button" variant="outline" onClick={onBack}>
        {backLabel}
      </Button>
      {action ?? (
        <Button type="button" variant="stream" onClick={onCreate} disabled={ctaDisabled}>
          {rendering ? <span aria-hidden className="studio-spinner" /> : null}
          {ctaLabel}
        </Button>
      )}
    </footer>
  );
}
