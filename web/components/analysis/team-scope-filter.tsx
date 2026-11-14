'use client';

import { useMemo, type ReactNode } from 'react';
import type { LibraryMatch, LibraryTeam } from '@/lib/api/library';
import { lastResults } from '@/lib/analysis/context-bar';
import type { ScopeField } from '@/lib/analysis/scope';
import type { TacticOption } from '@/lib/analysis/scope-tactics';
import { matchHref, teamHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { rivalOf, teamLabel, teamOutcome } from '@/lib/library/partidas';
import type { MapCounts } from '@/lib/analysis/map-counts';
import type { TeamAnalysis } from '@/hooks/use-team-analysis';
import type { TeamScopeControl } from '@/hooks/use-team-scope-control';
import { LatestLineupGap } from './latest-lineup-gap';
import { ScopeFilter, type ScopeFilterForm } from './scope-filter';
import type { ScopeCounts, ScopeEntity, ScopeMatch } from './scope-bar';

export type TeamScopeFilterProps = {
  form: ScopeFilterForm;
  control: TeamScopeControl;
  analysis: TeamAnalysis;
  /** Every library team, for the picker; null while loading. */
  teams: readonly LibraryTeam[] | null;
  tactics: readonly TacticOption[] | null;
  fields: readonly ScopeField[];
  /** The view reads one map at a time. */
  mapRequired?: boolean;
  limits?: { readonly options: readonly number[]; readonly fallback: number };
  mapCounts: MapCounts | null;
  /** The sample the view reads; undefined hides it, null shows it counting. */
  counts?: ScopeCounts | null;
  /** The matches behind the sample, for the dots and the "Mostrar partidas" list. */
  sampleMatches?: readonly LibraryMatch[];
  /** Another team was picked. */
  onTeam: (teamId: string) => void;
  className?: string;
};

/** The EDGE scope controls of one team, wired to the library data the team-scoped tools read. */
export function TeamScopeFilter(props: TeamScopeFilterProps): ReactNode {
  const { form, control, analysis, teams, tactics, fields, mapRequired, limits, mapCounts, counts, sampleMatches, onTeam, className } = props;
  const { team, matches, lineups, profile } = analysis;
  const entity = useMemo<ScopeEntity>(
    () => ({
      noun: 'Equipo',
      tone: 'team',
      id: team.id,
      options: [{ value: team.id, label: team.name }, ...(teams ?? []).filter((entry) => entry.id !== team.id).map((entry) => ({ value: entry.id, label: entry.name }))],
      recents: true,
      hrefFor: teamHref,
      onSelect: onTeam,
    }),
    [onTeam, team.id, team.name, teams],
  );
  const maps = useMemo(() => [...new Set(matches.map((match) => match.map))].sort(), [matches]);
  const opponents = useMemo(() => {
    const rivals = new Map<string, string>();
    for (const match of matches) {
      const rival = rivalOf(match, team.id);
      if (rival?.team_id) rivals.set(rival.team_id, teamLabel(rival));
    }
    return [...rivals].map(([value, label]) => ({ value, label })).sort((a, b) => a.label.localeCompare(b.label, 'es'));
  }, [matches, team.id]);
  const sample = useMemo(() => {
    if (sampleMatches === undefined) return {};
    const results = lastResults(
      sampleMatches.map((match) => ({ playedAt: match.played_at, outcome: teamOutcome(match, team.id) })),
      10,
    );
    const listed: ScopeMatch[] = sampleMatches.map((match) => ({
      id: match.id,
      href: matchHref(match.id),
      label: `${prettyMapName(match.map)} contra ${teamLabel(rivalOf(match, team.id))}`,
      date: formatShortDate(match.played_at),
      outcome: teamOutcome(match, team.id),
    }));
    return { results, matches: listed };
  }, [sampleMatches, team.id]);

  return (
    <>
      <ScopeFilter
        form={form}
        scope={control.scope}
        onChange={control.changeScope}
        onReset={control.resetScope}
        fields={fields}
        entity={entity}
        lineups={lineups}
        maps={maps}
        mapCounts={mapCounts}
        mapRequired={mapRequired}
        opponents={opponents}
        limits={limits}
        tactics={tactics ?? []}
        ownTeams={profile?.own_teams ?? 0}
        counts={counts}
        results={sample.results}
        matches={sample.matches}
        className={className}
      />
      {form === 'stack' ? <LatestLineupGap control={control} analysis={analysis} fields={fields} mapCounts={mapCounts} /> : null}
    </>
  );
}
