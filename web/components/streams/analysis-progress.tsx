'use client';

import type { ReactNode } from 'react';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { useElapsedSeconds } from '@/components/streams/use-elapsed-seconds';
import { cn } from '@/lib/utils';

/**
 * The waiting surface for the two background analyses (killfeed frames, speech
 * candidates). It is the same shape as the kit's `LongOperation` — stage label,
 * indeterminate track, elapsed clock — but composed here rather than reused,
 * because the whole thing has to be ONE `role="status"` region that also
 * contains the cancel control: the release E2E reads this element's text and
 * clicks the button inside it. Nesting `LongOperation` would publish a second
 * live region inside the first.
 *
 * The bar is indeterminate on purpose. Neither analysis reports progress, and
 * design.md forbids inventing one.
 */
export function StreamAnalysisProgress({
  label,
  onCancel,
  className,
}: {
  /** What is being analysed, e.g. "Analizando clips por fotograma". */
  label: string;
  onCancel: () => void;
  className?: string;
}): ReactNode {
  const elapsed = useElapsedSeconds(true);

  return (
    <div
      role="status"
      aria-live="polite"
      className={cn('flex min-w-56 flex-1 flex-col gap-2 border border-stream/35 bg-stream/[0.06] p-3', className)}
    >
      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1">
        <span className="font-mono text-meta uppercase tracking-wider text-stream-text">{label}</span>
        <span className="font-mono text-meta tabular-nums text-fg-3">{elapsed}s</span>
      </div>

      <Progress indeterminate size="xs" tone="stream" aria-label={label} />

      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-body-sm text-fg-2">
          {elapsed}s transcurridos · tiempo restante ajustándose
        </p>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          CANCELAR ESPERA
        </Button>
      </div>
    </div>
  );
}
