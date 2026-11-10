'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ListVideo } from 'lucide-react';
import { getTeamRounds, type LibraryMatch, type LibraryRosterEntry, type TeamCriterion } from '@/lib/api/library';
import { REPLAY_MAX_REFS, replayHref, type ReplayRef } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { teamReplayEntries } from '@/lib/analysis/team-replay-fields';
import type { MapCounts } from '@/lib/analysis/map-counts';
import type { ReplayForm } from '@/lib/replay/forms';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { PatternFinder } from '@/components/patterns/pattern-finder';
import { TeamGrenades } from '@/components/patterns/team-grenades';
import { ReplayFormTabs } from '@/components/replay/replay-frame';
import { Button } from '@/components/ui/button';
import { TeamSpatial } from './team-spatial';

/** The forms a team's rounds are read in; one round at a time opens from any figure as a playlist. */
export const TEAM_REPLAY_FORMS = ['calor', 'recorridos', 'granadas', 'patrones'] as const satisfies readonly ReplayForm[];
export type TeamReplayForm = (typeof TEAM_REPLAY_FORMS)[number];

/** "Ver 42 rondas en Repetición": the rounds the figure on screen counts, as one playlist, newest match first. */
function RoundsLink({ refs }: { refs: readonly ReplayRef[] | null }): ReactNode {
  if (refs === null || refs.length === 0) return null;
  const count = refs.length;
  let label = `Ver ${count} ${count === 1 ? 'ronda' : 'rondas'} en Repetición`;
  if (count > REPLAY_MAX_REFS) label = `Ver las ${REPLAY_MAX_REFS} primeras de ${count} rondas`;
  return (
    <Button asChild variant="ghost" size="xs" className="mb-1">
      <Link href={replayHref(refs, 'Rondas del equipo')}>
        <ListVideo aria-hidden />
        {label}
      </Link>
    </Button>
  );
}

/** The grenades form reads one map, so its playlist asks the rounds endpoint for that map alone. */
function useGrenadeRounds({ teamId, scope, lineup, criteria, active }: {
  teamId: string;
  scope: AnalysisScope;
  lineup: readonly string[];
  criteria: readonly TeamCriterion[];
  active: boolean;
}): ReplayRef[] | null {
  const [now] = useState(() => Date.now());
  const [refs, setRefs] = useState<ReplayRef[] | null>(null);
  const { from, to } = scopeDateRange(scope.period, now);
  const { map, side, opponent } = scope;

  useEffect(() => {
    setRefs(null);
    if (!active || map === '') return;
    let live = true;
    getTeamRounds(teamId, { maps: [map], side, from, to, opponent_ids: opponent === '' ? [] : [opponent], lineup, criteria })
      .then((data) => {
        if (live) setRefs(data.matches.flatMap((match) => match.rounds.map((round) => ({ matchId: match.match_id, round: round.round }))));
      })
      .catch(() => {
        if (live) setRefs(null);
      });
    return () => {
      live = false;
    };
  }, [teamId, map, side, from, to, opponent, lineup, criteria, active]);
  return refs;
}

/** The team's Replay: its rounds as a heatmap, as paths or as grenades, through the page's scope. */
export function TeamReplay({
  teamId,
  scope,
  lineup,
  matches,
  mapCounts,
  roster,
  form,
  onFormChange,
  onMapChange,
  onScopeChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  matches: readonly LibraryMatch[];
  /** Matches per map in the scope, for the map select of the heatmap and the paths. */
  mapCounts: MapCounts | null;
  /** The players a heatmap or the paths can count alone. */
  roster: readonly LibraryRosterEntry[];
  form: TeamReplayForm;
  onFormChange: (form: TeamReplayForm) => void;
  onMapChange: (map: string) => void;
  onScopeChange: (patch: Partial<AnalysisScope>) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const criteria = useMemo(() => teamReplayEntries(scope), [scope]);
  const [spatialRefs, setSpatialRefs] = useState<readonly ReplayRef[] | null>(null);
  const onRounds = useCallback((refs: readonly ReplayRef[]) => setSpatialRefs(refs), []);
  const grenadeRefs = useGrenadeRounds({ teamId, scope, lineup, criteria, active: form === 'granadas' });
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-end gap-2">
        <div className="min-w-0 flex-1">
          <ReplayFormTabs
            forms={TEAM_REPLAY_FORMS}
            value={form}
            onChange={(next) => {
              const known = TEAM_REPLAY_FORMS.find((entry) => entry === next);
              if (known !== undefined) onFormChange(known);
            }}
          />
        </div>
        {form === 'patrones' ? null : <RoundsLink refs={form === 'granadas' ? grenadeRefs : spatialRefs} />}
      </div>
      {form === 'patrones' ? (
        <PatternFinder teamId={teamId} scope={scope} lineup={lineup} criteria={criteria} matches={matches} roster={roster} onMapChange={onMapChange} onScopeChange={onScopeChange} onCounts={onCounts} />
      ) : null}
      {form === 'granadas' ? (
        <TeamGrenades
          teamId={teamId}
          scope={scope}
          lineup={lineup}
          criteria={criteria}
          matches={matches}
          onMapChange={onMapChange}
          onCounts={onCounts}
        />
      ) : null}
      {form === 'calor' || form === 'recorridos' ? (
        <TeamSpatial
          teamId={teamId}
          scope={scope}
          lineup={lineup}
          criteria={criteria}
          view={form === 'calor' ? 'heatmap' : 'overlay'}
          roster={roster}
          mapCounts={mapCounts}
          onMapChange={onMapChange}
          onCounts={onCounts}
          onRounds={onRounds}
        />
      ) : null}
    </div>
  );
}
