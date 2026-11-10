'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getTeamFirstDeath, type FirstDeathMovement, type FirstDeathPlayer, type FirstDeathZone, type TeamFirstDeath } from '@/lib/api/first-death';
import type { TeamSpatialQuery } from '@/lib/api/library';
import type { TacticalSide } from '@/lib/api/tactical';
import { mapRoleChips } from '@/lib/analysis/map-role';
import { evidenceRefs } from '@/lib/analysis/playbook';
import { findingHref } from '@/lib/analysis/replay-link';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { FIRST_DEATH_TRAMOS, type FirstDeathTramoId, type Tramo } from '@/lib/replay/tramo';
import { placeLabel } from '@/lib/tactical-labels';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { Button } from '@/components/ui/button';
import { RoleChip } from '@/components/ui/role-chip';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { playerColor } from './ct-positions-map';
import { FirstDeathMap } from './first-death-map';
import { PrepSection, roleText } from './prep-sections';

const MOVEMENTS: readonly { value: FirstDeathMovement; label: string }[] = [
  { value: 'static', label: 'Estático' },
  { value: 'slow', label: 'Lento' },
  { value: 'fast', label: 'Rápido' },
];
const SIDES: readonly { value: TacticalSide; label: string }[] = [
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
];
/** The cards shown at once; stand-ins and rarer players wait behind "otros". */
const MAIN_PLAYERS = 5;
const SECONDS = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export type FirstDeathPositionsProps = {
  teamId: string;
  /** The team scope the bar keeps, map and lineup included. */
  query: TeamSpatialQuery & { map: string };
  /** The bar's side; empty lets the section pick one. */
  scopeSide: '' | TacticalSide;
};

type Picked = { steamId: string; callout: string };

/** "Posiciones en T hasta la primera muerte": each player's five zones with most time, and one zone's samples on the map. */
export function FirstDeathPositions({ teamId, query, scopeSide }: FirstDeathPositionsProps): ReactNode {
  const [ownSide, setOwnSide] = useState<TacticalSide>('T');
  const side = scopeSide === '' ? ownSide : scopeSide;
  const [tramoId, setTramoId] = useState<FirstDeathTramoId>('opening');
  const [movement, setMovement] = useState<FirstDeathMovement>('static');
  const [data, setData] = useState<TeamFirstDeath | null | 'failed'>(null);
  const [picked, setPicked] = useState<Picked | null>(null);
  const [showMap, setShowMap] = useState(true);
  const tramo = FIRST_DEATH_TRAMOS.find((entry) => entry.id === tramoId)?.tramo ?? FIRST_DEATH_TRAMOS[0].tramo;

  useEffect(() => {
    let active = true;
    setData(null);
    getTeamFirstDeath(teamId, { ...query, side, from_seconds: tramo.from, to_seconds: tramo.to, movement })
      .then((report) => {
        if (active) setData(report);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, query, side, tramo.from, tramo.to, movement]);

  const controls = (
    <div className="flex flex-wrap items-center gap-2 print:hidden">
      {scopeSide === '' ? <Segmented label="Lado" value={ownSide} options={SIDES} onChange={(value) => { setOwnSide(value); setPicked(null); }} /> : null}
      <Segmented label="Tramo" value={tramoId} options={FIRST_DEATH_TRAMOS.map((entry) => ({ value: entry.id, label: entry.label }))} onChange={setTramoId} />
      <Segmented label="Movimiento" value={movement} options={MOVEMENTS} onChange={setMovement} />
    </div>
  );
  return (
    <PrepSection title={`Posiciones en ${side} hasta la primera muerte`} hint="Cuenta cada ronda hasta la primera muerte de la ronda, de cualquier jugador.">
      {controls}
      <FirstDeathBody teamId={teamId} tramo={tramo} data={data} picked={picked} onPick={setPicked} showMap={showMap} onToggleMap={() => setShowMap((value) => !value)} />
    </PrepSection>
  );
}

type BodyProps = {
  teamId: string;
  /** The tramo the links open the Replay on. */
  tramo: Tramo;
  data: TeamFirstDeath | null | 'failed';
  picked: Picked | null;
  onPick: (picked: Picked) => void;
  showMap: boolean;
  onToggleMap: () => void;
};

function FirstDeathBody({ teamId, tramo, data, picked, onPick, showMap, onToggleMap }: BodyProps): ReactNode {
  const [showOthers, setShowOthers] = useState(false);
  if (data === 'failed') return <p role="alert" className="text-body-sm text-destructive">No se pudieron leer las posiciones. Comprueba que ClipHub Studio sigue abierto.</p>;
  if (data === null) return <Skeleton className="h-48 w-full" />;
  if (data.players.length === 0) {
    return (
      <p className="text-body-sm text-fg-2">
        {data.round_count === 0 ? `Sin rondas de ${data.side} con este filtro.` : 'Estas partidas no guardan posiciones de los jugadores.'}
      </p>
    );
  }
  const index = picked === null ? -1 : data.players.findIndex((player) => player.steamid64 === picked.steamId);
  const player = data.players[index];
  const zone = player?.zones.find((entry) => entry.callout === picked?.callout);
  const matchName = new Map(data.match_list.map((match) => [match.match_id, match.opponent === '' ? formatShortDate(match.played_at) : match.opponent]));
  const shown = showOthers ? data.players : data.players.slice(0, MAIN_PLAYERS);
  const others = data.players.length - MAIN_PLAYERS;
  const link = (refs: FirstDeathZone['refs'], title: string): string =>
    player === undefined || zone === undefined ? '' : findingHref({ refs: evidenceRefs(refs), title, focus: [player.steamid64], zones: [zone.callout], tramo });
  return (
    <div className="flex flex-col gap-3">
      <p className="text-meta text-fg-3">Segundos por ronda con posiciones, leídos en {data.framed} rondas de {data.side} con posiciones.</p>
      <div className="grid gap-2 sm:grid-cols-2 @[90rem]/content:grid-cols-5">
        {shown.map((entry, position) => (
          <PlayerCard key={entry.steamid64} player={entry} side={data.side} index={position} picked={picked} onPick={onPick} />
        ))}
      </div>
      {others > 0 ? (
        <Button type="button" variant="ghost" size="xs" className="self-start print:hidden" aria-expanded={showOthers} onClick={() => setShowOthers((value) => !value)}>
          {showOthers ? 'Ocultar otros' : `otros (${others})`}
        </Button>
      ) : null}
      <section aria-label="Posiciones en el mapa" className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <h3 className="text-body-sm font-semibold text-fg-1">Posiciones en el mapa</h3>
          <Button type="button" variant="ghost" size="xs" onClick={onToggleMap} className="print:hidden">
            {showMap ? 'Ocultar mapa' : 'Mostrar mapa'}
          </Button>
        </div>
        {showMap && (player === undefined || zone === undefined) ? <p className="text-body-sm text-fg-2">Elige un jugador y una zona</p> : null}
        {showMap && player !== undefined && zone !== undefined ? (
          <div className="flex flex-col gap-2 lg:flex-row lg:items-start">
            <FirstDeathMap teamId={teamId} map={data.map} zone={zone} color={playerColor(index)} label={`${player.name} en ${prettyMapName(data.map)}`} />
            <div className="flex min-w-0 flex-col gap-1">
              <p className="text-body-sm text-fg-1">
                {player.name} en {placeLabel(zone.callout)}: {zone.rounds} {zone.rounds === 1 ? 'ronda' : 'rondas'}
              </p>
              <Button asChild variant="outline" size="xs" className="self-start">
                <Link href={link(zone.refs, `${data.team_name} · ${player.name} en ${placeLabel(zone.callout)}`)}>Ver las rondas en Repetición</Link>
              </Button>
              <ul aria-label="Rondas" className="flex flex-wrap gap-1">
                {zone.refs.map((ref) => (
                  <li key={`${ref.match_id}-${ref.round}`}>
                    <Link href={link([ref], `${player.name} en ${placeLabel(zone.callout)}`)} className="text-meta text-fg-2 tabular-nums hover:text-primary hover:underline">
                      {matchName.get(ref.match_id) ?? 'Partida'}, ronda {ref.round}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        ) : null}
      </section>
    </div>
  );
}

type PlayerCardProps = { player: FirstDeathPlayer; side: TacticalSide; index: number; picked: Picked | null; onPick: (picked: Picked) => void };

function PlayerCard({ player, side, index, picked, onPick }: PlayerCardProps): ReactNode {
  const chips = mapRoleChips(side === 'T' ? { t: player.map_role } : { ct: player.map_role }, side);
  const library = roleText(player.role);
  return (
    <article aria-label={player.name} className="studio-panel flex flex-col gap-2 px-3 py-2.5">
      <header className="flex items-center gap-2">
        <PlayerAvatar nickname={player.name} playerID={player.steamid64} size={32} />
        <div className="flex min-w-0 flex-col">
          <h4 className="flex items-center gap-1.5 truncate text-body font-semibold text-fg-1">
            <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ backgroundColor: playerColor(index) }} />
            {player.name}
          </h4>
          {chips.length === 0 && library === '' ? null : (
            <p className="flex flex-wrap items-center gap-1">
              {chips.map((chip) => (
                <RoleChip key={chip.side} side={chip.side} label={chip.label} />
              ))}
              {library === '' ? null : <RoleChip label={library} />}
            </p>
          )}
        </div>
      </header>
      <p className="text-meta text-fg-2">Rondas con posiciones {player.rounds}</p>
      {player.zones.length === 0 ? (
        <p className="text-body-sm text-fg-3">Sin datos</p>
      ) : (
        <ol className="flex flex-col gap-0.5">
          {player.zones.map((zone, rank) => {
            const active = picked?.steamId === player.steamid64 && picked.callout === zone.callout;
            return (
              <li key={zone.callout}>
                <button
                  type="button"
                  aria-pressed={active}
                  onClick={() => onPick({ steamId: player.steamid64, callout: zone.callout })}
                  className="flex w-full items-baseline gap-2 rounded-sm px-1 text-left text-body-sm hover:bg-surface-2 aria-pressed:bg-surface-3"
                >
                  <span className="w-3 shrink-0 text-fg-3 tabular-nums">{rank + 1}</span>
                  <span className="min-w-0 flex-1 truncate text-fg-1">{placeLabel(zone.callout)}</span>
                  <span className="shrink-0 text-fg-2 tabular-nums">{SECONDS.format(zone.seconds)} s</span>
                </button>
              </li>
            );
          })}
        </ol>
      )}
    </article>
  );
}
