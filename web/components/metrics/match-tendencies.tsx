'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { usePathname } from 'next/navigation';
import { fetchTacticalDocument } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalSource } from '@/lib/api/tactical';
import { MATCH_REPLAY_TAB } from '@/lib/library/rounds';
import { tacticalFilterToQuery } from '@/lib/tactical-filter';
import { TacticalTendenciesPanel } from '@/components/tactical/tactical-tendencies';
import { TendencyLinksProvider } from '@/components/tactical/tendency-links';
import { useTeamTendencies } from '@/components/tactical/use-team-tendencies';

const NO_FILTER: TacticalFilter = {};

/**
 * What each team did in the match, under its metrics: wins by buy, attack and
 * defence shapes, sites, timings and opening duels. Every figure opens its
 * rounds in the Replay.
 */
export function MatchTendencies({ matchId }: { matchId: string }): ReactNode {
  const pathname = usePathname();
  const source = useMemo<TacticalSource>(() => ({ kind: 'library', id: matchId }), [matchId]);
  const [doc, setDoc] = useState<TacticalDocument | null>(null);
  useEffect(() => {
    let active = true;
    setDoc(null);
    // A match without analysis shows no tendencies.
    fetchTacticalDocument(source).then(
      (next) => {
        if (active) setDoc(next);
      },
      () => undefined,
    );
    return () => {
      active = false;
    };
  }, [source]);
  const tendencies = useTeamTendencies({ source, filter: NO_FILTER, doc });
  const base = tendencies.tendencies?.filter;
  const roundsHref = useCallback(
    (patch: TacticalFilter) => `${pathname}?tab=${MATCH_REPLAY_TAB}&${tacticalFilterToQuery({ ...base, ...patch })}`,
    [base, pathname],
  );
  if (doc === null) return null;
  return (
    <TendencyLinksProvider roundsHref={roundsHref}>
      <TacticalTendenciesPanel {...tendencies} roundCount={doc.rounds.length} filtered={false} />
    </TendencyLinksProvider>
  );
}
