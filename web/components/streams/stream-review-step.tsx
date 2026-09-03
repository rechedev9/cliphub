'use client';

import type { ReactNode } from 'react';
import type { CreativeBriefItem } from '@/lib/reel-brief';
import { cn } from '@/lib/utils';

export const BRIEF_APPROVAL_LABEL = 'Apruebo el brief antes de renderizar';

/** Step 05: the exact brief the render will honor, and the one checkbox that approves it. */
export function StreamReviewStep({
  items,
  approved,
  approvable,
  blockerHint,
  busy,
  onApprovedChange,
}: {
  items: CreativeBriefItem[];
  approved: boolean;
  approvable: boolean;
  /** What still blocks approval; shown instead of the checkbox hint while `approvable` is false. */
  blockerHint: string | null;
  busy: boolean;
  onApprovedChange: (approved: boolean) => void;
}): ReactNode {
  return (
    <>
      <p className="text-body-sm text-fg-2">
        Esto es exactamente lo que se renderiza. Cualquier cambio en un paso anterior vuelve a pedir la aprobación.
      </p>
      <dl className="studio-panel flex flex-col gap-2 px-3.5 py-3 text-body-sm">
        {items.map((item) => (
          <div key={item.label} className="flex min-w-0 items-baseline justify-between gap-3">
            <dt className="shrink-0 font-mono text-meta uppercase tracking-wider text-fg-3">{item.label}</dt>
            <dd className="min-w-0 truncate text-right text-fg-1" title={item.value}>
              {item.value}
            </dd>
          </div>
        ))}
      </dl>
      <label
        className={cn(
          'flex min-h-11 items-center gap-3 border px-3.5 py-2 font-display text-label font-semibold uppercase tracking-wide transition-colors duration-(--dur-fast)',
          approved ? 'border-success/45 bg-success/10 text-success' : 'border-border-strong bg-surface-2 text-fg-1',
          !approvable && 'opacity-50',
        )}
      >
        <input
          type="checkbox"
          checked={approved}
          disabled={busy || !approvable}
          onChange={(event) => onApprovedChange(event.target.checked)}
          className="size-[18px] shrink-0 cursor-pointer accent-success disabled:cursor-not-allowed"
        />
        {BRIEF_APPROVAL_LABEL}
      </label>
      {blockerHint === null ? null : <p className="text-body-sm text-warning">{blockerHint}</p>}
    </>
  );
}
