'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import type { ScoutEvidence } from '@/lib/api/scout';
import type { SpatialMatchRef } from '@/lib/api/spatial';
import { prettyMapName } from '@/lib/format';
import { evidencePath } from '@/lib/reports/scout-sections';
import { cn } from '@/lib/utils';

/**
 * A row of mutually exclusive buttons. The active one is marked with the
 * accent edge, never filled with it.
 */
export function Segmented<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly { value: T; label: string; disabled?: boolean }[];
  onChange: (value: T) => void;
}): ReactNode {
  return (
    <div role="group" aria-label={label} className="inline-flex flex-wrap rounded-md border border-border-strong bg-surface-1 p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={value === option.value}
          disabled={option.disabled}
          onClick={() => onChange(option.value)}
          className={cn(
            'h-8 rounded-sm px-2.5 text-body-sm whitespace-nowrap text-fg-2 outline-none transition-colors',
            'hover:text-fg-1 focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-45',
            value === option.value && 'bg-surface-3 font-medium text-fg-1 shadow-[inset_0_-2px_0_0_var(--primary)]',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

/** The short name of an example round: the map when the view spans matches, then the round. */
export function evidenceShortLabel(evidence: ScoutEvidence, matches: ReadonlyMap<string, SpatialMatchRef>, many: boolean): string {
  const match = matches.get(evidence.match_id);
  const prefix = many && match ? `${match.opponent ? `vs ${match.opponent}` : prettyMapName(match.map)} ` : '';
  return `${prefix}R${evidence.round}`;
}

/** Links to the rounds and ticks a number came from. */
export function EvidenceChips({
  examples,
  matches,
}: {
  examples: readonly ScoutEvidence[];
  matches: ReadonlyMap<string, SpatialMatchRef>;
}): ReactNode {
  const many = matches.size > 1;
  return (
    <span className="flex flex-wrap gap-1">
      {examples.map((evidence) => (
        <Link
          key={`${evidence.match_id}:${evidence.round}:${evidence.tick}`}
          href={evidencePath(evidence)}
          className="rounded-sm border border-border px-1.5 text-meta whitespace-nowrap text-fg-2 hover:border-primary hover:text-primary"
        >
          {evidenceShortLabel(evidence, matches, many)}
        </Link>
      ))}
    </span>
  );
}
