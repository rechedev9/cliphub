'use client';

import { useEffect, useState } from 'react';
import { fetchTacticalTendencies } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalFilter, TacticalSource, TacticalTendencies } from '@/lib/api/tactical';
import { analysisErrorMessage } from '@/lib/replay/loaded-analysis';
import { tacticalFilterCount, tacticalFilterFromQuery, tacticalFilterToQuery } from '@/lib/tactical-filter';
import { panelFilter, panelTeamKey } from '@/lib/tactical-tendency-view';
import type { TendencyTeamChoice } from '@/components/tactical/tactical-tendencies';

type Fetched = { key: string; tendencies: TacticalTendencies | null; error: string | null };

export type TeamTendencies = {
  tendencies: TacticalTendencies | null;
  error: string | null;
  stale: boolean;
  team: TendencyTeamChoice | undefined;
  filtered: boolean;
};

/**
 * The tendencies under the replay, read from one team: the filter's team, or the
 * one the panel's toggle picks on top of the filter. The URL never sees the toggle.
 */
export function useTeamTendencies({
  source,
  filter,
  doc,
}: {
  source: TacticalSource;
  filter: TacticalFilter;
  /** Null while the analysis loads; without it only a filter that names a team can be fetched. */
  doc: TacticalDocument | null;
}): TeamTendencies {
  const [chosen, setChosen] = useState<string | undefined>(undefined);
  const [fetched, setFetched] = useState<Fetched | null>(null);
  const { kind, id } = source;

  const teamKey = doc === null ? filter.team_key : panelTeamKey({ teams: doc.teams, rounds: doc.rounds, filter, chosen });
  const query = doc === null && filter.team_key === undefined ? null : tacticalFilterToQuery(panelFilter(filter, teamKey));
  const key = query === null ? null : `${kind}:${id}?${query}`;

  useEffect(() => {
    if (key === null || query === null) return;
    let active = true;
    fetchTacticalTendencies({ kind, id }, tacticalFilterFromQuery(new URLSearchParams(query)))
      .then((next) => {
        if (active) setFetched({ key, tendencies: next, error: null });
      })
      .catch((error: unknown) => {
        if (active) setFetched({ key, tendencies: null, error: analysisErrorMessage(error) });
      });
    return () => {
      active = false;
    };
  }, [kind, id, query, key]);

  const stale = fetched === null || fetched.key !== key;
  const team: TendencyTeamChoice | undefined =
    doc === null || doc.teams.length < 2 || teamKey === undefined
      ? undefined
      : {
          teams: doc.teams.map((entry) => ({ key: entry.key, name: entry.name || entry.key })),
          selected: teamKey,
          locked: filter.team_key !== undefined,
          onSelect: setChosen,
        };

  return {
    tendencies: fetched?.tendencies ?? null,
    // An error belongs to the request that failed; the next one starts clean.
    error: stale ? null : (fetched?.error ?? null),
    stale,
    team,
    filtered: tacticalFilterCount({ ...filter, team_key: undefined }) > 0,
  };
}
