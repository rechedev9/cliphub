'use client';

import { useState, type ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import type { CentreRound } from '@/lib/replay/centre-round';
import { ROUND_OUTCOME_LABEL, roundOutcome } from '@/lib/replay/round-outcome';
import { buyLabel, ctPatternLabel, siteLabel, tPatternLabel } from '@/lib/tactical-labels';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** A card of the column that folds; the count says what is inside while it is closed. */
function Fold({ title, count, defaultOpen, children }: { title: string; count?: number; defaultOpen: boolean; children: ReactNode }): ReactNode {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <section className="flex flex-col rounded-sm border border-border-subtle bg-surface-0" aria-label={title}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        className={cn('flex h-8 items-center gap-1.5 rounded-sm px-2 text-meta font-semibold text-fg-2 outline-none hover:text-fg-1', FOCUS_RING)}
      >
        <ChevronRight className={cn('size-3.5 transition-transform duration-(--dur-fast)', open && 'rotate-90')} aria-hidden />
        {title}
        {count === undefined || count === 0 ? null : <span className="tabular-nums text-fg-3">{count}</span>}
      </button>
      {open ? <div className="border-t border-border-subtle p-2">{children}</div> : null}
    </section>
  );
}

/** What this round was: both buys, both shapes, the site and how it ended. */
function RoundContext({ round }: { round: CentreRound }): ReactNode {
  const { economy, class: shape, end_reason: endReason } = round.round;
  const rows: [string, ReactNode][] = [
    ['Compra', <>
      <span className="text-side-ct">{buyLabel(economy.ct_buy)}</span>
      <span className="text-fg-3"> vs </span>
      <span className="text-side-t">{buyLabel(economy.t_buy)}</span>
    </>],
    ['Forma', <>
      <span className="text-side-t">{tPatternLabel(shape.t_side)}</span>
      <span className="text-fg-3"> · </span>
      <span className="text-side-ct">{ctPatternLabel(shape.ct_side)}</span>
    </>],
    ['Sitio', siteLabel(shape.site)],
    ['Final', ROUND_OUTCOME_LABEL[roundOutcome(endReason)]],
  ];
  return (
    <dl className="grid grid-cols-[3.75rem_minmax(0,1fr)] gap-x-2 gap-y-1 text-body-sm">
      {rows.map(([label, value]) => (
        <div key={label} className="contents">
          <dt className="text-meta leading-5 text-fg-3">{label}</dt>
          <dd className="truncate text-fg-1">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

/**
 * The left column: the round in a few facts and what to make of it, the filters and round types the
 * page passes in, and the round's key moments.
 */
export function ReplayLeftPanel({
  focused,
  actions,
  filters,
  moments,
  comms,
}: {
  focused: CentreRound | null;
  /** The team voice's comms of a library match; null without a recording. */
  comms?: ReactNode;
  /** What can be made from this round: a video, a play. */
  actions: ReactNode;
  filters: ReactNode;
  /** The round's key moments as jump buttons; null with the layer off. */
  moments: { count: number; list: ReactNode } | null;
}): ReactNode {
  const reasons = focused?.round.class.reasons ?? [];
  return (
    <div className="flex flex-col gap-2">
      {focused === null ? null : (
        <Fold title="Contexto de la ronda" defaultOpen>
          <RoundContext round={focused} />
        </Fold>
      )}
      {actions}
      {filters === undefined || filters === null ? null : <div className="rounded-sm border border-border-subtle bg-surface-0 p-2">{filters}</div>}
      {moments === null ? null : (
        <Fold title="Momentos clave" count={moments.count} defaultOpen>
          {moments.list}
        </Fold>
      )}
      {comms === undefined || comms === null ? null : (
        <Fold title="Comunicaciones" defaultOpen>
          {comms}
        </Fold>
      )}
      {reasons.length === 0 ? null : (
        <Fold title="Por qué se clasificó así" defaultOpen>
          <ul className="flex flex-col gap-1 font-mono text-meta break-words text-fg-2">
            {reasons.map((reason) => (
              <li key={reason}>{reason}</li>
            ))}
          </ul>
        </Fold>
      )}
    </div>
  );
}
