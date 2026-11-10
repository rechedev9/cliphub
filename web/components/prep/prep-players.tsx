'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { getTeamSideStats } from '@/lib/api/library';
import type { TeamSideStats } from '@/lib/api/side-stats';
import { PlayerStatCard } from '@/components/teams/player-stat-card';
import { Skeleton } from '@/components/ui/skeleton';
import { PrepSection } from './prep-sections';

export type PrepPlayersScope = {
  side: '' | 'CT' | 'T';
  /** Empty counts every map. */
  map: string;
  from?: string;
  to?: string;
  opponentIds: readonly string[];
  lineup: readonly string[];
};

/** Preparar's "Jugadores": a card per player over the same rounds as the rest of the tab (the `team_side_stats` figures). */
export function PrepPlayers({ teamId, scope }: { teamId: string; scope: PrepPlayersScope }): ReactNode {
  const [data, setData] = useState<TeamSideStats | null | 'failed'>(null);
  const { side, map, from, to, opponentIds, lineup } = scope;
  const opponentKey = opponentIds.join(',');

  useEffect(() => {
    let active = true;
    setData(null);
    getTeamSideStats(teamId, {
      maps: map === '' ? [] : [map],
      side,
      from,
      to,
      opponent_ids: opponentKey === '' ? [] : opponentKey.split(','),
      lineup,
    })
      .then((report) => {
        if (active) setData(report);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, from, to, opponentKey, lineup]);

  return (
    <PrepSection title="Jugadores">
      <PlayersBody data={data} side={side} />
    </PrepSection>
  );
}

function PlayersBody({ data, side }: { data: TeamSideStats | null | 'failed'; side: PrepPlayersScope['side'] }): ReactNode {
  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron leer los jugadores. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (data === null) return <Skeleton className="h-32 w-full" />;
  if (data.players.length === 0) return <p className="text-body-sm text-fg-2">Sin rondas analizadas con este filtro.</p>;
  return (
    <ul className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-2">
      {data.players.map((player) => (
        <PlayerStatCard key={player.steamid64} player={player} side={side} />
      ))}
    </ul>
  );
}
