'use client';

import { useCallback, useMemo, useState, type ReactNode } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { PlugZap, Shield } from 'lucide-react';
import { sampleNotes } from '@/lib/api/team-insights';
import { lineupSteamIds } from '@/lib/analysis/lineup';
import { REPLAY_MAX_REFS } from '@/lib/analysis/replay-link';
import { TEAM_REPLAY_LIMITS, TEAM_REPLAY_STACK_FIELDS } from '@/lib/analysis/team-replay-fields';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { matchTitle } from '@/lib/library/partidas';
import { withTeam } from '@/lib/nav-scope';
import { entryKey, indexOfKey } from '@/lib/replay/playlist';
import { TeamChoice } from '@/components/analysis/team-choice';
import { TeamScopeFilter, type TeamScopeFilterProps } from '@/components/analysis/team-scope-filter';
import { EmptyState } from '@/components/records/empty-state';
import { useDocumentTitle } from '@/components/shell/route-title';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { useTeamAnalysis, useTeamList, useTeamTactics, type TeamAnalysis } from '@/hooks/use-team-analysis';
import { useTeamMapCounts } from '@/hooks/use-team-map-counts';
import { useTeamRounds } from '@/hooks/use-team-rounds';
import { useTeamScopeControl } from '@/hooks/use-team-scope-control';
import { ReplayWorkspace } from './replay-workspace';

/** The team's 2D replay: loads the team, then hands over to `TeamReplayReady`. */
export function TeamReplayStage({ teamId }: { teamId: string }): ReactNode {
  const { state, reload } = useTeamAnalysis(teamId);
  const teams = useTeamList();
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams().toString();
  const onTeam = useCallback((id: string) => router.replace(withTeam({ pathname, search }, id), { scroll: false }), [pathname, router, search]);

  if (state.kind === 'loading') return <Skeleton role="status" aria-label="Cargando el equipo" className="h-[60vh] w-full rounded-lg" />;
  if (state.kind === 'missing') {
    return (
      <div className="flex flex-col gap-4">
        <EmptyState icon={Shield} title="Equipo no encontrado" description="Puede que se haya fusionado con otro equipo o que el enlace sea antiguo." />
        <TeamChoice sentence="Elige otro equipo para ver sus rondas." teams={teams} onPick={onTeam} />
      </div>
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="Servicio local sin conexión"
        description="No se pudo leer el equipo. Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={reload}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <TeamReplayReady analysis={state.data} teams={teams} onTeam={onTeam} />;
}

function matchLabelOf(analysis: TeamAnalysis): (matchId: string) => string | null {
  const byId = new Map(analysis.matches.map((match) => [match.id, match]));
  return (matchId) => {
    const match = byId.get(matchId);
    return match === undefined ? null : `${prettyMapName(match.map)} · ${matchTitle(match)} · ${formatShortDate(match.played_at)}`;
  };
}

/**
 * The rounds of a team scope as one list in the 2D replay: the filter stack floats over the map, the
 * rounds that match fill the round row, and MULTI RONDA superposes them. The rounds come from the
 * endpoint the team record's Replay tab reads.
 */
function TeamReplayReady({ analysis, teams, onTeam }: { analysis: TeamAnalysis; teams: TeamScopeFilterProps['teams']; onTeam: (id: string) => void }): ReactNode {
  const { team, matches, lineups } = analysis;
  const tactics = useTeamTactics(team.id);
  const control = useTeamScopeControl({ teamId: team.id, tactics, matches, defaultMap: true });
  const { scope } = control;
  const lineupKey = lineupSteamIds(scope.lineup, lineups).join(',');
  const lineup = useMemo(() => (lineupKey === '' ? [] : lineupKey.split(',')), [lineupKey]);
  const mapCounts = useTeamMapCounts({ teamId: team.id, scope, fields: TEAM_REPLAY_STACK_FIELDS, lineup, enabled: true });
  const read = useTeamRounds({ teamId: team.id, scope, lineup });
  const [currentKey, setCurrentKey] = useState<string | null>(null);
  useDocumentTitle(`Replay 2D · ${team.name} · ClipHub`);

  const entries = useMemo(() => read.refs?.refs ?? [], [read.refs]);
  const index = indexOfKey(entries, currentKey);
  const groupLabel = useMemo(() => matchLabelOf(analysis), [analysis]);
  const matchDate = useCallback(
    (matchId: string) => {
      const match = matches.find((candidate) => candidate.id === matchId);
      return match === undefined ? null : formatShortDate(match.played_at);
    },
    [matches],
  );
  const matchIds = useMemo(() => new Set(read.refs?.matchIds ?? []), [read.refs]);
  const sampleMatches = useMemo(() => matches.filter((match) => matchIds.has(match.id)), [matches, matchIds]);
  const counts = read.refs === null || read.loading ? null : { maps: read.refs.matchIds.length, rounds: read.refs.total, notes: sampleNotes(read.skipped, read.awpEstimated) };
  const capped = read.refs !== null && read.refs.total > REPLAY_MAX_REFS;
  const stackSummary = [team.name, scope.map === '' ? null : prettyMapName(scope.map), `${entries.length} ${entries.length === 1 ? 'ronda' : 'rondas'}`]
    .filter((part) => part !== null)
    .join(' · ');

  const stack = (
    <div className="flex w-72 max-w-full flex-col gap-2">
      <TeamScopeFilter
        form="stack"
        control={control}
        analysis={analysis}
        teams={teams}
        tactics={tactics}
        fields={TEAM_REPLAY_STACK_FIELDS}
        mapRequired
        limits={TEAM_REPLAY_LIMITS}
        mapCounts={mapCounts}
        counts={read.refs === null && scope.map === '' ? undefined : counts}
        sampleMatches={sampleMatches}
        onTeam={onTeam}
      />
      {capped ? (
        <p className="text-meta text-fg-3">
          Se cargan las {REPLAY_MAX_REFS} primeras de {read.refs?.total} rondas. Acota el filtro para ver el resto.
        </p>
      ) : null}
    </div>
  );

  if (entries.length === 0) {
    let message = 'Ninguna ronda cumple el filtro. Quita alguno para volver a ver rondas.';
    if (scope.map === '') message = 'Este equipo aún no tiene partidas analizadas con las que llenar el replay.';
    else if (read.failed) message = 'No se pudieron leer las rondas del equipo. Comprueba que ClipHub Studio sigue abierto y cambia un filtro para reintentar.';
    else if (read.loading || read.refs === null) message = 'Leyendo las rondas del equipo…';
    return (
      <section className="studio-panel grid items-start gap-5 p-4 sm:p-5 lg:grid-cols-[18rem_minmax(0,1fr)]" aria-label="Repetición">
        {stack}
        <p role="status" className="self-center text-body text-fg-2">
          {message}
        </p>
      </section>
    );
  }

  return (
    <ReplayWorkspace
      source="library"
      entries={entries}
      index={index}
      onIndexChange={(next) => setCurrentKey(entries[next] === undefined ? null : entryKey(entries[next]))}
      groupLabel={groupLabel}
      matchDate={matchDate}
      title={team.name}
      stack={{ summary: stackSummary, content: stack }}
    />
  );
}
