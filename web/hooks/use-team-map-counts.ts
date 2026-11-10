'use client';

import { useEffect, useMemo, useState } from 'react';
import { getTeamMapCounts } from '@/lib/api/library';
import { mapCountsOf, mapCountsQuery, type MapCounts } from '@/lib/analysis/map-counts';
import type { AnalysisScope, ScopeField } from '@/lib/analysis/scope';

export type TeamMapCountsOptions = {
  teamId: string;
  scope: AnalysisScope;
  fields: readonly ScopeField[];
  lineup: readonly string[];
};

/**
 * Matches per map under the scope the view reads, counted by the server so the
 * map selector agrees with the view; null while counting or when it fails, and
 * the selector then shows no counts.
 */
export function useTeamMapCounts({ teamId, scope, fields, lineup }: TeamMapCountsOptions): MapCounts | null {
  const [counts, setCounts] = useState<MapCounts | null>(null);
  const [now] = useState(() => Date.now());
  const reads = fields.includes('map');
  const key = useMemo(() => JSON.stringify(mapCountsQuery({ scope, fields, lineup, now })), [scope, fields, lineup, now]);

  useEffect(() => {
    if (!reads) return;
    let active = true;
    // JSON.parse yields a plain query again; it was stringified from the typed one above.
    const query: ReturnType<typeof mapCountsQuery> = JSON.parse(key);
    getTeamMapCounts(teamId, query)
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
