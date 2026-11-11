'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import {
  FIRST_DEATH_MOVEMENTS,
  getTeamFirstDeath,
  type FirstDeathMovement,
  type FirstDeathPlayer,
  type FirstDeathZone,
  type TeamFirstDeath,
} from '@/lib/api/first-death';
import { LIBRARY_ROLES } from '@/lib/api/library';
import type { TacticalSide } from '@/lib/api/tactical';
import { mapRoleChips } from '@/lib/analysis/map-role';
import { evidenceRefs } from '@/lib/analysis/playbook';
import { findingHref } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { FIRST_DEATH_TRAMOS, type FirstDeathTramoId } from '@/lib/replay/tramo';
import { placeLabel } from '@/lib/tactical-labels';
import { InfoTip } from '@/components/analysis/info-tip';
import { CAPS_LABEL } from '@/components/analysis/scope-field-style';
import { FirstDeathMap } from '@/components/prep/first-death-map';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { SlashTitle } from '@/components/records/document-page';
import { Button } from '@/components/ui/button';
import { RoleChip } from '@/components/ui/role-chip';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const MOVEMENT_LABEL: Record<FirstDeathMovement, string> = { static: 'Estático', slow: 'Lento', fast: 'Rápido' };
const SECONDS = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
/** The cards shown at once; stand-ins and rarer players wait behind "otros". */
const MAIN_PLAYERS = 6;
const CHART_COLORS = 7;

function playerColor(index: number): string {
  return `var(--chart-${(index % CHART_COLORS) + 1})`;
}

function libraryRole(role: string): string {
  const known = LIBRARY_ROLES.find((entry) => entry === role);
  return known === undefined || known === 'unknown' ? '' : ROLE_LABEL[known];
}

function ChipRow<T extends string>({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: T;
  options: readonly { value: T; label: string }[];
  onChange: (value: T) => void;
}): ReactNode {
  return (
    <div role="group" aria-label={label} className="flex flex-wrap items-center gap-1.5">
      <span className={cn(CAPS_LABEL, 'mr-1 shrink-0')}>{label}</span>
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          aria-pressed={value === option.value}
          onClick={() => onChange(option.value)}
          className={cn(
            'h-6 border border-border-strong px-2.5 text-body-sm text-fg-2 tabular-nums hover:border-fg-4 hover:text-fg-1',
            'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
            value === option.value && 'border-primary bg-primary/14 font-semibold text-primary hover:border-primary hover:text-primary',
          )}
        >
          {option.label}
        </button>
      ))}
    </div>
  );
}

type Picked = { steamId: string; callout: string };

function PlayerCard({ player, side, index, picked, onPick }: {
  player: FirstDeathPlayer;
  side: TacticalSide;
  index: number;
  picked: Picked | null;
  onPick: (picked: Picked) => void;
}): ReactNode {
  const chips = mapRoleChips(side === 'T' ? { t: player.map_role } : { ct: player.map_role }, side);
  const library = libraryRole(player.role);
  return (
    <li className="flex min-w-0 flex-col gap-2 border border-border bg-surface-2 p-3">
      <div className="flex items-center gap-2">
        <PlayerAvatar nickname={player.name} playerID={player.steamid64} size={32} />
        <div className="flex min-w-0 flex-col gap-0.5">
          <p className="flex items-center gap-1.5 truncate text-body font-bold text-fg-1">
            <span aria-hidden className="size-2 shrink-0 rounded-full" style={{ backgroundColor: playerColor(index) }} />
            {player.name}
          </p>
          {chips.length === 0 && library === '' ? null : (
            <p className="flex flex-wrap items-center gap-1">
              {chips.map((chip) => (
                <RoleChip key={chip.side} side={chip.side} label={chip.label} />
              ))}
              {library === '' || chips.length > 0 ? null : <RoleChip label={library} />}
            </p>
          )}
        </div>
      </div>
      <p className="flex items-baseline justify-between gap-2 border-y border-border-subtle py-1.5">
        <span className={CAPS_LABEL}>Rondas jugadas</span>
        <span className="text-body font-bold text-fg-1 tabular-nums">{player.rounds}</span>
      </p>
      {player.zones.length === 0 ? (
        <p className="text-body-sm text-fg-3">Sin datos</p>
      ) : (
        <ol aria-label={`Zonas de ${player.name}`} className="flex flex-col gap-0.5">
          {player.zones.map((zone, rank) => (
            <li key={zone.callout}>
              <button
                type="button"
                aria-pressed={picked?.steamId === player.steamid64 && picked.callout === zone.callout}
                onClick={() => onPick({ steamId: player.steamid64, callout: zone.callout })}
                className="flex w-full items-baseline gap-2 rounded-sm px-1 py-0.5 text-left text-body-sm hover:bg-surface-3 aria-pressed:bg-surface-5"
              >
                <span className="w-3 shrink-0 text-fg-3 tabular-nums">{rank + 1}</span>
                <span className="min-w-0 flex-1 truncate text-fg-1">{placeLabel(zone.callout)}</span>
                <span className="shrink-0 text-fg-2 tabular-nums">{SECONDS.format(zone.seconds)} s</span>
              </button>
            </li>
          ))}
        </ol>
      )}
    </li>
  );
}

type PanelProps = { teamId: string; data: TeamFirstDeath; picked: Picked | null; tramo: { from: number; to: number } };

function MapPanel({ teamId, data, picked, tramo }: PanelProps): ReactNode {
  const index = picked === null ? -1 : data.players.findIndex((player) => player.steamid64 === picked.steamId);
  const player = data.players[index];
  const zone: FirstDeathZone | undefined = player?.zones.find((entry) => entry.callout === picked?.callout);
  const names = new Map(data.match_list.map((match) => [match.match_id, match.opponent === '' ? formatShortDate(match.played_at) : match.opponent]));
  return (
    <section aria-label="Posiciones en el mapa" className="flex flex-col gap-2">
      <SlashTitle>Posiciones en el mapa</SlashTitle>
      {player === undefined || zone === undefined ? (
        <div className="flex flex-col gap-2">
          <p className="w-fit border border-border bg-surface-3 px-2 py-1 text-meta text-fg-2">Ningún jugador ni zona elegidos</p>
          <p className="grid min-h-40 place-items-center border border-border bg-surface-2 text-body-sm text-fg-3">Sin datos.</p>
        </div>
      ) : (
        <div className="flex flex-col gap-2 lg:flex-row lg:items-start">
          <FirstDeathMap teamId={teamId} map={data.map} zone={zone} color={playerColor(index)} label={`${player.name} en ${prettyMapName(data.map)}`} />
          <div className="flex min-w-0 flex-col gap-1.5">
            <p className="text-body-sm text-fg-1">
              {player.name} en {placeLabel(zone.callout)}: {zone.rounds} {zone.rounds === 1 ? 'ronda' : 'rondas'}
            </p>
            <Button asChild variant="outline" size="xs" className="self-start">
              <Link href={findingHref({ refs: evidenceRefs(zone.refs), title: `${data.team_name} · ${player.name} en ${placeLabel(zone.callout)}`, focus: [player.steamid64], zones: [zone.callout], tramo })}>
                Ver las rondas en Repetición
              </Link>
            </Button>
            <ul aria-label="Rondas" className="flex flex-wrap gap-x-3 gap-y-1">
              {zone.refs.map((ref) => (
                <li key={`${ref.match_id}-${ref.round}`}>
                  <Link
                    href={findingHref({ refs: evidenceRefs([ref]), title: `${player.name} en ${placeLabel(zone.callout)}`, focus: [player.steamid64], zones: [zone.callout], tramo })}
                    className="text-meta text-fg-2 tabular-nums hover:text-primary hover:underline"
                  >
                    {names.get(ref.match_id) ?? 'Partida'}, ronda {ref.round}
                  </Link>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
    </section>
  );
}

export type TeamPositionsProps = {
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
};

/**
 * Posiciones: where each player stands until the first death of the round, by time window and by how
 * fast they move, from the first-death endpoint the Preparar tab reads.
 */
export function TeamPositions({ teamId, scope, lineup }: TeamPositionsProps): ReactNode {
  const [ownSide, setOwnSide] = useState<TacticalSide>('T');
  const side: TacticalSide = scope.side === '' ? ownSide : scope.side;
  const [tramoId, setTramoId] = useState<FirstDeathTramoId>('opening');
  const [movement, setMovement] = useState<FirstDeathMovement>('static');
  const [data, setData] = useState<TeamFirstDeath | null | 'failed'>(null);
  const [picked, setPicked] = useState<Picked | null>(null);
  const [showOthers, setShowOthers] = useState(false);
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);
  const tramo = (FIRST_DEATH_TRAMOS.find((entry) => entry.id === tramoId) ?? FIRST_DEATH_TRAMOS[0]).tramo;
  const opponentKey = scope.opponent;
  const lineupKey = lineup.join(',');
  const query = useMemo(
    () => ({
      map: scope.map,
      from,
      to,
      lineup: lineupKey === '' ? [] : lineupKey.split(','),
      opponent_ids: opponentKey === '' ? [] : [opponentKey],
    }),
    [scope.map, from, to, lineupKey, opponentKey],
  );

  useEffect(() => {
    let active = true;
    setData(null);
    setPicked(null);
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

  if (scope.map === '') {
    return <p className="border border-border bg-surface-2 px-3 py-2 text-body-sm text-fg-2">Elige un mapa para ver dónde está cada jugador hasta la primera muerte.</p>;
  }
  const players = data === null || data === 'failed' ? [] : data.players;
  const shown = showOthers ? players : players.slice(0, MAIN_PLAYERS);
  const others = Math.max(0, players.length - MAIN_PLAYERS);
  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center gap-2">
        <SlashTitle>Posiciones en {side} (hasta la primera muerte)</SlashTitle>
        {data !== null && data !== 'failed' && data.players.length > 0 ? (
          <InfoTip
            label="las posiciones"
            text={`Segundos por ronda con posiciones, leídos en ${data.framed} rondas de ${data.side} con posiciones. Cuenta cada ronda hasta la primera muerte de la ronda, de cualquier jugador.`}
          />
        ) : null}
      </div>
      <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
        {scope.side === '' ? (
          <ChipRow
            label="Lado"
            value={ownSide}
            options={[{ value: 'T', label: 'T' }, { value: 'CT', label: 'CT' }]}
            onChange={setOwnSide}
          />
        ) : null}
        <ChipRow label="Tiempo" value={tramoId} options={FIRST_DEATH_TRAMOS.map((entry) => ({ value: entry.id, label: entry.label }))} onChange={setTramoId} />
        <ChipRow label="Movimiento" value={movement} options={FIRST_DEATH_MOVEMENTS.map((value) => ({ value, label: MOVEMENT_LABEL[value] }))} onChange={setMovement} />
      </div>
      {data === 'failed' ? (
        <p role="alert" className="text-body-sm text-destructive">No se pudieron leer las posiciones. Comprueba que ClipHub Studio sigue abierto.</p>
      ) : null}
      {data === null ? <Skeleton role="status" aria-label="Leyendo las posiciones" className="h-48 w-full" /> : null}
      {data !== null && data !== 'failed' && data.players.length === 0 ? (
        <p className="text-body-sm text-fg-2">
          {data.round_count === 0 ? `Sin rondas de ${data.side} con este filtro.` : 'Estas partidas no guardan posiciones de los jugadores.'}
        </p>
      ) : null}
      {data !== null && data !== 'failed' && data.players.length > 0 ? (
        <>
          <ul className="grid grid-cols-[repeat(auto-fill,minmax(9.5rem,1fr))] gap-2">
            {shown.map((player, index) => (
              <PlayerCard key={player.steamid64} player={player} side={data.side} index={index} picked={picked} onPick={setPicked} />
            ))}
          </ul>
          {others > 0 ? (
            <Button type="button" variant="ghost" size="xs" className="self-start" aria-expanded={showOthers} onClick={() => setShowOthers((value) => !value)}>
              {showOthers ? 'Ocultar otros' : `otros (${others})`}
            </Button>
          ) : null}
          <MapPanel teamId={teamId} data={data} picked={picked} tramo={tramo} />
        </>
      ) : null}
    </div>
  );
}
