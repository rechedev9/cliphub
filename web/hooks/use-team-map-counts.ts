'use client';

import { useEffect, useRef, useState } from 'react';
import { getTeamMapCounts } from '@/lib/api/library';
import { mapCountsOf, mapCountsQuery, type MapCounts } from '@/lib/analysis/map-counts';
import type { AnalysisScope, ScopeField } from '@/lib/analysis/scope';

export type TeamMapCountsOptions = {
  teamId: string;
  scope: AnalysisScope;
  fields: readonly ScopeField[];
  lineup: readonly string[];
};

/** Matches per map under the scope the view reads, counted by the server; null while counting or on failure. */
export function useTeamMapCounts({ teamId, scope, fields, lineup }: TeamMapCountsOptions): MapCounts | null {
  const [counts, setCounts] = useState<MapCounts | null>(null);
  const [now] = useState(() => Date.now());
  const reads = fields.includes('map');
  const query = mapCountsQuery({ scope, fields, lineup, now });
  const key = JSON.stringify(query);
  // The effect reruns on the key, not on the object a render rebuilds.
  const latest = useRef(query);
  useEffect(() => {
    latest.current = query;
  });

  useEffect(() => {
    if (!reads) return;
    let active = true;
    getTeamMapCounts(teamId, latest.current)
      .then((response) => {
        if (active) setCounts(mapCountsOf(response));
      })
      .catch(() => {
        if (active) setCounts(null);
      });
    return () => {
      active = false;
    };
  }, [teamId, key, reads]);

  return reads ? counts : null;
}
