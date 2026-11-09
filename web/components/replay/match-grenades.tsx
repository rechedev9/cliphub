'use client';

import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Bomb, PlugZap } from 'lucide-react';
import { isLibraryOffline, listTeamGrenades } from '@/lib/api/library';
import type { LibraryMatch } from '@/lib/api/library';
import type { TeamGrenades } from '@/lib/api/grenades';
import type { TacticalDocument, TacticalFilter, TacticalRound } from '@/lib/api/tactical';
import { prettyMapName } from '@/lib/format';
import { matchTitle } from '@/lib/library/partidas';
import { EmptyState } from '@/components/records/empty-state';
import { GrenadeFinder } from '@/components/patterns/grenade-finder';
import type { TramoControl } from './tramo-bar';
import type { PlayerFocus } from '@/lib/replay/player-focus';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';

type Load = { kind: 'loading' } | { kind: 'ready'; data: TeamGrenades } | { kind: 'offline' } | { kind: 'failed' };

/**
 * The Granadas form of one partida: what the chosen team threw in the rounds
 * the scope kept. The grenades belong to a team, so the form asks for one when
 * the scope has both.
 */
export function MatchGrenades({
  match,
  doc,
  filter,
  rounds,
  onPickTeam,
  tramo,
  focus,
}: {
  match: LibraryMatch;
  doc: TacticalDocument;
  filter: TacticalFilter;
  /** The rounds the scope kept. */
  rounds: readonly TacticalRound[];
  onPickTeam: (teamKey: string) => void;
  tramo: TramoControl;
  /** The players whose throws count; empty for the whole team. */
  focus: PlayerFocus;
}): ReactNode {
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const team = match.teams.find((entry) => entry.key === filter.team_key);
  const teamId = team?.team_id ?? null;
  const { side } = filter;

  useEffect(() => {
    if (teamId === null) return;
    let active = true;
    setLoad({ kind: 'loading' });
    listTeamGrenades(teamId, { maps: [match.map], match_ids: [match.id], side: side ?? '' })
      .then((data) => {
        if (active) setLoad({ kind: 'ready', data });
      })
      .catch((error: unknown) => {
        if (active) setLoad({ kind: isLibraryOffline(error) ? 'offline' : 'failed' });
      });
    return () => {
      active = false;
    };
  }, [teamId, match.id, match.map, side, attempt]);

  // The server reads the match and the side; the rest of the scope narrows the rounds here.
  const kept = useMemo(() => new Set(rounds.map((round) => round.number)), [rounds]);
  // The Replay's focus is the finder's player choice, with the same colours on the radar and the clock.
  const throwers = useMemo(() => doc.players.filter((player) => focus.includes(player.slot)).map((player) => player.steamid64), [doc.players, focus]);
  const data = useMemo<TeamGrenades | null>(() => {
    if (load.kind !== 'ready') return null;
    return {
      ...load.data,
      grenades: load.data.grenades.filter((grenade) => kept.has(grenade.round)),
      round_list: load.data.round_list.filter((round) => kept.has(round.round)),
      rounds: kept.size,
    };
  }, [load, kept]);
  const matchLabel = useCallback(() => matchTitle(match), [match]);
  const plants = useMemo(() => {
    const out = new Map<number, number>();
    for (const round of rounds) {
      const plant = round.bomb?.plant_tick ?? 0;
      if (plant > round.tick_freeze_end && doc.demo.tickrate > 0) out.set(round.number, (plant - round.tick_freeze_end) / doc.demo.tickrate);
    }
    return out;
  }, [rounds, doc.demo.tickrate]);
  const plantSeconds = useCallback((_: string, round: number) => plants.get(round) ?? null, [plants]);

  if (team === undefined) {
    return (
      <EmptyState
        icon={Bomb}
        title="Elige un equipo"
        description="Las granadas se leen de un equipo cada vez."
        actions={
          <div className="flex flex-wrap gap-2">
            {doc.teams.map((entry) => (
              <Button key={entry.key} type="button" variant="outline" size="sm" onClick={() => onPickTeam(entry.key)}>
                {entry.name || entry.key}
              </Button>
            ))}
          </div>
        }
      />
    );
  }
  if (teamId === null) {
    return (
      <EmptyState
        icon={Bomb}
        title="Este equipo no está en Equipos"
        description="Asígnale un equipo en el Resumen de la partida, con Corregir, y sus granadas aparecerán aquí."
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
  if (data === null) {
    return (
      <div role="status" aria-label="Cargando granadas" className="grid gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(20rem,1fr)]">
        <Skeleton className="aspect-square w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (data.grenades.length === 0) {
    return (
      <EmptyState
        icon={Bomb}
        title="Ninguna granada en estas rondas"
        description={`${team.team_name ?? team.clan_name} no tiró granadas en las rondas de ${prettyMapName(match.map)} que deja el filtro.`}
      />
    );
  }
  return (
    <GrenadeFinder
      key={`${teamId}|${match.map}`}
      map={match.map}
      geometry={doc.geometry}
      data={data}
      buy=""
      matchLabel={matchLabel}
      tramo={tramo}
      plantSeconds={plantSeconds}
      players={throwers}
      teamId={teamId}
    />
  );
}
