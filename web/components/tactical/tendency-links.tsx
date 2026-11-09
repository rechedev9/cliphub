'use client';

import { createContext, useContext, type ReactNode } from 'react';
import Link from 'next/link';
import type { TacticalFilter } from '@/lib/api/tactical';
import { cn } from '@/lib/utils';

/** Turns the filter of one figure into the Replay link of its rounds; null where the figures do not link. */
type RoundsHref = ((patch: TacticalFilter) => string) | null;

const TendencyLinks = createContext<RoundsHref>(null);

export function TendencyLinksProvider({ roundsHref, children }: { roundsHref: RoundsHref; children: ReactNode }): ReactNode {
  return <TendencyLinks value={roundsHref}>{children}</TendencyLinks>;
}

/** A figure that opens its rounds in the Replay; plain text where there is nowhere to open them. */
export function RoundsLink({ patch, className, children }: { patch: TacticalFilter; className?: string; children: ReactNode }): ReactNode {
  const roundsHref = useContext(TendencyLinks);
  if (roundsHref === null) return <span className={className}>{children}</span>;
  return (
    <Link
      href={roundsHref(patch)}
      title="Ver estas rondas en el Replay"
      className={cn('rounded-xs underline decoration-fg-4 underline-offset-2 outline-none hover:decoration-fg-1 focus-visible:ring-[3px] focus-visible:ring-ring/50', className)}
    >
      {children}
    </Link>
  );
}
