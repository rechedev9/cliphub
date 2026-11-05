'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import type { ScoutEvidence } from '@/lib/api/scout';
import type { SpatialMatchRef } from '@/lib/api/spatial';
import { prettyMapName } from '@/lib/format';
import { evidencePath } from '@/lib/reports/scout-sections';

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
