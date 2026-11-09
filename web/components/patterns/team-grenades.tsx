'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { Map as MapIcon, PlugZap } from 'lucide-react';
import { isLibraryOffline, listTeamGrenades, type LibraryMatch, type TeamCriterion } from '@/lib/api/library';
import { unrankedCount } from '@/lib/api/team-insights';
import type { TeamGrenades as TeamGrenadesData } from '@/lib/api/grenades';
import { HALF_BUY, scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { rivalOf, teamLabel } from '@/lib/library/partidas';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { GrenadeFinder } from './grenade-finder';

const NO_CRITERIA: readonly TeamCriterion[] = [];

type Load = { kind: 'loading' } | { kind: 'ready'; data: TeamGrenadesData } | { kind: 'offline' } | { kind: 'failed' };

function FinderSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando granadas" className="grid gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,1fr)]">
      <div className="flex flex-col gap-3">
        <Skeleton className="h-9 w-full max-w-md" />
        <Skeleton className="aspect-square w-full" />
        <Skeleton className="h-24 w-full" />
      </div>
      <div className="flex flex-col gap-2">
        <Skeleton className="h-10 w-full" />
        {Array.from({ length: 6 }, (_, index) => (
          <Skeleton key={index} className="h-16 w-full" />
        ))}
      </div>
    </div>
  );
}

/**
 * The "Granadas" tab: every grenade the team threw on one map, on the radar
 * and grouped by spot, read through the page's analysis scope.
 */
export function TeamGrenades({
  teamId,
  scope,
  lineup,
  criteria = NO_CRITERIA,
  matches,
  onMapChange,
  onCounts,
}: {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  /** The rest of the team Replay's filters bar, as query entries. */
  criteria?: readonly TeamCriterion[];
  matches: readonly LibraryMatch[];
  onMapChange: (map: string) => void;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [now] = useState(() => Date.now());
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const { from, to } = scopeDateRange(scope.period, now);
  const { map, side, opponent } = scope;
  const geometry = useMapGeometry(teamId, map === '' ? [] : [map]).get(map);

  useEffect(() => {
    if (map === '') {
      onCounts({ maps: 0, rounds: 0 });
      return;
    }
    let active = true;
    setLoad({ kind: 'loading' });
    onCounts(null);
    listTeamGrenades(teamId, { maps: [map], side, from, to, opponent_ids: opponent === '' ? [] : [opponent], lineup, criteria })
      .then((data) => {
        if (!active) return;
        setLoad({ kind: 'ready', data });
        onCounts({ maps: new Set(data.grenades.map((grenade) => grenade.match_id)).size, rounds: data.rounds, unrated: unrankedCount(data.skipped) });
      })
      .catch((error: unknown) => {
        if (active) setLoad({ kind: isLibraryOffline(error) ? 'offline' : 'failed' });
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, from, to, opponent, lineup, criteria, attempt, onCounts]);

  const matchLabel = useCallback(
    (matchId: string): string => {
      const match = matches.find((entry) => entry.id === matchId);
      return match === undefined ? 'Partida' : `vs ${teamLabel(rivalOf(match, teamId))} ${formatShortDate(match.played_at)}`;
    },
    [matches, teamId],
  );

  if (map === '') {
    const maps = [...new Set(matches.map((match) => match.map))].sort();
    return (
      <EmptyState
        icon={MapIcon}
        title="Elige un mapa"
        description="Las granadas se dibujan sobre el radar de un mapa."
        actions={
          maps.length === 0 ? undefined : (
            <div className="flex flex-wrap gap-2">
              {maps.map((entry) => (
                <Button key={entry} type="button" variant="outline" size="sm" onClick={() => onMapChange(entry)}>
                  {prettyMapName(entry)}
                </Button>
              ))}
            </div>
          )
        }
      />
    );
  }
  if (load.kind === 'offline' || load.kind === 'failed') {
    return (
      <EmptyState
        icon={PlugZap}
        title={load.kind === 'offline' ? 'Servicio local sin conexión' : 'No se pudieron leer las granadas'}
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  if (load.kind === 'loading' || geometry === undefined) return <FinderSkeleton />;
  if (geometry === null) {
    return (
      <EmptyState
        icon={MapIcon}
        title="Sin radar para este mapa"
        description={`Ninguna partida de ${prettyMapName(map)} tiene un análisis con el que dibujar el radar.`}
      />
    );
  }
  return (
    <GrenadeFinder
      key={map}
      map={map}
      geometry={geometry}
      data={load.data}
      buy={scope.buy === HALF_BUY ? '' : scope.buy}
      matchLabel={matchLabel}
    />
  );
}
