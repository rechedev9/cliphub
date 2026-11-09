'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { ListVideo } from 'lucide-react';
import { getTeamRounds, type LibraryMatch, type LibraryRosterEntry, type TeamCriterion } from '@/lib/api/library';
import { replayHref, type ReplayRef } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope, type ScopeField } from '@/lib/analysis/scope';
import { teamScopeEntries } from '@/lib/analysis/team-scope';
import type { ReplayForm } from '@/lib/replay/forms';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { TeamGrenades } from '@/components/patterns/team-grenades';
import { ReplayFormTabs } from '@/components/replay/replay-frame';
import { Button } from '@/components/ui/button';
import { TeamSpatial } from './team-spatial';

/** The forms a team's rounds are read in; one round at a time opens from any figure as a playlist. */
export const TEAM_REPLAY_FORMS = ['calor', 'recorridos', 'granadas'] as const satisfies readonly ReplayForm[];
export type TeamReplayForm = (typeof TEAM_REPLAY_FORMS)[number];

/** The filters bar of the team's Replay, in the order it shows them; every one is applied by the server. */
export const TEAM_REPLAY_FIELDS: readonly ScopeField[] = [
  'lineup', 'side', 'period', 'limit', 'buy', 'opponentBuy', 'awp', 'opponentAwp', 'advantage', 'rounds', 'opponent', 'opponentTop', 'whose',
];
/** The match counts the Replay offers; with none in the URL it reads the server's cap. */
export const TEAM_REPLAY_LIMITS = { options: [3, 5, 10, 20, 50], fallback: 50 } as const;

type RoundsLink = { href: string; count: number } | null;

/** "Ver 42 rondas en Repetición": the kept rounds of every match as one playlist, newest match first. */
function TeamRoundsLink({ teamId, scope, lineup, criteria }: { teamId: string; scope: AnalysisScope; lineup: readonly string[]; criteria: readonly TeamCriterion[] }): ReactNode {
  const [now] = useState(() => Date.now());
  const [link, setLink] = useState<RoundsLink>(null);
  const { from, to } = scopeDateRange(scope.period, now);
  const { map, side, opponent } = scope;

  useEffect(() => {
    let active = true;
    setLink(null);
    getTeamRounds(teamId, { maps: map === '' ? [] : [map], side, from, to, opponent_ids: opponent === '' ? [] : [opponent], lineup, criteria })
      .then((data) => {
        if (!active) return;
        const refs: ReplayRef[] = data.matches.flatMap((match) => match.rounds.map((round) => ({ matchId: match.match_id, round: round.round })));
        setLink(refs.length === 0 ? null : { href: replayHref(refs, `Rondas de ${data.team_name}`), count: refs.length });
      })
      .catch(() => {
        if (active) setLink(null);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, from, to, opponent, lineup, criteria]);

  if (link === null) return null;
  return (
    <Button asChild variant="ghost" size="xs" className="mb-1">
      <Link href={link.href}>
        <ListVideo aria-hidden />
        Ver {link.count} {link.count === 1 ? 'ronda' : 'rondas'} en Repetición
      </Link>
    </Button>
  );
}

/** The team's Replay: its rounds as a heatmap, as paths or as grenades, through the page's scope. */
export function TeamReplay({
  teamId,
  scope,
  lineup,
  matches,
  roster,
  form,
  onFormChange,
  onMapChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  matches: readonly LibraryMatch[];
  /** The players a heatmap or the paths can count alone. */
  roster: readonly LibraryRosterEntry[];
  form: TeamReplayForm;
  onFormChange: (form: TeamReplayForm) => void;
  onMapChange: (map: string) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const criteria = useMemo(() => teamScopeEntries(scope, TEAM_REPLAY_FIELDS), [scope]);
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
        <TeamRoundsLink teamId={teamId} scope={scope} lineup={lineup} criteria={criteria} />
      </div>
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
      ) : (
        <TeamSpatial
          teamId={teamId}
          scope={scope}
          lineup={lineup}
          criteria={criteria}
          view={form === 'calor' ? 'heatmap' : 'overlay'}
          roster={roster}
          onMapChange={onMapChange}
          onCounts={onCounts}
        />
      )}
    </div>
  );
}
