'use client';
import type { ReactNode } from 'react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/components/ui/button';
export function StreamFooter({
  error,
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
  error: string | null;
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
      {error ? (
        <p role="alert" className="flex basis-full items-start gap-2 text-body-sm text-destructive">
          <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0" />
          {error}
        </p>
      ) : null}
      {/* A floor on the summary pushes the buttons to their own row on phones
          instead of squeezing "1 Short · 01 · 0:12" into four lines. */}
      <div className="min-w-[12rem] flex-1">
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
