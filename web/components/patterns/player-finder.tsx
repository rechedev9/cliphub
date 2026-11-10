'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Flame, Layers, Map as MapIcon, PlugZap, X } from 'lucide-react';
import { getSpatialHeatmap, getSpatialOverlay, type LibraryRosterEntry, type TeamCriterion, type TeamSpatialQuery } from '@/lib/api/library';
import type { SpatialResponse } from '@/lib/api/spatial';
import { RADAR_LEVELS, type RadarLevel } from '@/lib/api/tactical';
import type { WorldBox } from '@/lib/analysis/grenade-patterns';
import { binsInBox, clockHistogram, firstMovements, roundsThroughBox } from '@/lib/analysis/pattern-canvas';
import { findingHref, type ReplayRef } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { heatBandwidth, heatDensity, SMOOTHING_DEFAULT, type HeatDensity } from '@/lib/heat-density';
import { prettyMapName } from '@/lib/format';
import { WHOLE_ROUND, isWholeRound } from '@/lib/replay/tramo';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap } from '@/lib/tactical-transform';
import { CAPS_LABEL } from '@/components/analysis/scope-field-style';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { EmptyState } from '@/components/records/empty-state';
import { TramoBar, useTramo, type TramoControl } from '@/components/replay/tramo-bar';
import { HeatLegend } from '@/components/tactical/spatial/heat-legend';
import { Button } from '@/components/ui/button';
import { Segmented } from '@/components/ui/segmented';
import { InfoTip } from '@/components/analysis/info-tip';
import { Skeleton } from '@/components/ui/skeleton';
import { Toggle } from '@/components/ui/toggle';
import { ClockHistogram, type HistogramKey } from './clock-histogram';
import { FramedParts, type FinderFrame, type FinderParts } from './finder-frame';
import { playerToken } from './grenade-players';
import { MovementList } from './movement-list';
import { PlayerHeatRadar, type PlayerHeat } from './player-heat-radar';

/** At most this many players at once: each one is a request and a colour. */
export const HEAT_MAX_PLAYERS = 5;
/** Waits this long after the last change before asking again, so a drag of the window asks once. */
const RUN_DELAY_MS = 250;
const TEAM_TOKEN = '--primary';

type Load<T> = { kind: 'loading' } | { kind: 'ready'; data: T } | { kind: 'failed' };

type Props = {
  frame: FinderFrame;
  teamId: string;
  scope: AnalysisScope;
  /** The scope's lineup as SteamID64s; empty counts every round. */
  lineup: readonly string[];
  criteria: readonly TeamCriterion[];
  roster: readonly LibraryRosterEntry[];
  tramo: TramoControl;
  /** The players in focus, by SteamID64, kept in the page's URL. */
  pick: { picked: readonly string[]; onChange: (next: readonly string[]) => void };
  matchLabel: (matchId: string) => string;
  /** The team's Replay on its heatmap form, where deaths, kills and utility maps live. */
  moreHeatHref: string;
  onCounts: (counts: ScopeCounts | null) => void;
};

/**
 * The Jugadores panel: where each chosen player stood on one map, a heat per player in their colour inside
 * a time window and a box, with the moment each one first moves in every round of the sample.
 */
export function PlayerFinder({ frame, teamId, scope, lineup, criteria, roster, tramo: tramoControl, pick, matchLabel, moreHeatHref, onCounts }: Props): ReactNode {
  const { tramo, onChange: setTramo } = useTramo(tramoControl);
  const [now] = useState(() => Date.now());
  const [box, setBox] = useState<WorldBox | null>(null);
  const [boxLevel, setBoxLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const [floor, setFloor] = useState<RadarLevel | 'both'>('both');
  const [overlay, setOverlay] = useState<Load<SpatialResponse>>({ kind: 'loading' });
  const [heat, setHeat] = useState<Load<readonly SpatialResponse[]>>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const { from, to } = scopeDateRange(scope.period, now);
  const { map, side, opponent } = scope;

  const players = useMemo(() => [...roster].sort((a, b) => b.last_played_at.localeCompare(a.last_played_at) || b.matches - a.matches), [roster]);
  const known = useMemo(() => new Set(players.map((player) => player.steamid64)), [players]);
  const picked = useMemo(() => pick.picked.filter((id) => known.has(id)).slice(0, HEAT_MAX_PLAYERS), [pick.picked, known]);
  const tokens = useMemo(() => new Map(players.map((player, index) => [player.steamid64, playerToken(index)])), [players]);
  const names = useMemo(() => new Map(players.map((player) => [player.steamid64, player.nick === '' ? player.steamid64 : player.nick])), [players]);
  const pickedKey = picked.join(',');
  const lineupKey = lineup.join(',');

  const query = useMemo<TeamSpatialQuery>(
    () => ({ map: map === '' ? undefined : map, side, from, to, opponent_ids: opponent === '' ? [] : [opponent], lineup: lineupKey === '' ? [] : lineupKey.split(','), criteria }),
    [map, side, from, to, opponent, lineupKey, criteria],
  );
  const fromTramo = isWholeRound(tramo) ? 0 : tramo.from;
  const toTramo = isWholeRound(tramo) ? 0 : tramo.to;
  const fromPlant = tramo.fromPlant === true;

  useEffect(() => {
    if (map === '') {
      onCounts({ maps: 0, rounds: 0 });
      return;
    }
    let live = true;
    setOverlay({ kind: 'loading' });
    getSpatialOverlay({ kind: 'team', teamId, query })
      .then((data) => {
        if (!live) return;
        setOverlay({ kind: 'ready', data });
        onCounts({ maps: data.matches.length, rounds: data.overlay?.total ?? 0 });
      })
      .catch(() => {
        if (live) setOverlay({ kind: 'failed' });
      });
    return () => {
      live = false;
    };
  }, [teamId, map, query, attempt, onCounts]);

  useEffect(() => {
    if (map === '') return;
    let live = true;
    const timer = window.setTimeout(() => {
      const groups: readonly (string | null)[] = pickedKey === '' ? [null] : pickedKey.split(',');
      Promise.all(
        groups.map((player) =>
          getSpatialHeatmap({ kind: 'team', teamId, query: { ...query, players: player === null ? [] : [player] } }, { kind: 'position', from: fromTramo, to: toTramo, fromPlant }),
        ),
      )
        .then((data) => {
          if (live) setHeat({ kind: 'ready', data });
        })
        .catch(() => {
          if (live) setHeat({ kind: 'failed' });
        });
    }, RUN_DELAY_MS);
    return () => {
      live = false;
      window.clearTimeout(timer);
    };
  }, [teamId, map, query, pickedKey, fromTramo, toTramo, fromPlant, attempt]);

  const overlayData = overlay.kind === 'ready' ? overlay.data : null;
  const geometry = overlayData?.geometry ?? null;
  const calibration = geometry?.calibration ?? null;
  const multiLevel = calibration !== null && isMultiLevelMap(calibration);
  const levels = useMemo<RadarLevel[]>(
    () => (multiLevel && floor === 'both' ? [RADAR_LEVELS.default, RADAR_LEVELS.lower] : [floor === 'both' ? RADAR_LEVELS.default : floor]),
    [multiLevel, floor],
  );

  const heats = useMemo((): PlayerHeat[] => {
    if (heat.kind !== 'ready' || calibration === null) return [];
    const groups: readonly (string | null)[] = pickedKey === '' ? [null] : pickedKey.split(',');
    const bandwidth = heatBandwidth(map, SMOOTHING_DEFAULT);
    return heat.data.flatMap((response, index) => {
      const heatmap = response.heatmap;
      if (heatmap === undefined) return [];
      const byLevel = new Map<RadarLevel, HeatDensity>();
      for (const level of [RADAR_LEVELS.default, RADAR_LEVELS.lower]) {
        // Without a side in the scope a player's CT and T rounds both count: the heat is where they were, not where one half put them.
        const sources = heatmap.density.filter((entry) => entry.level === level && (side === '' || entry.side === side));
        if (sources.length === 0) continue;
        const otherFloor = multiLevel && boxLevel !== level;
        let bins: readonly (readonly [number, number, number])[] = sources.flatMap((source) => source.bins);
        if (box !== null) bins = otherFloor ? [] : binsInBox({ bins, grid: heatmap.density_grid, calibration, box });
        const density = heatDensity({ grid: heatmap.density_grid, bins, bandwidth });
        if (density !== null) byLevel.set(level, density);
      }
      const owner = groups[index];
      return [{ token: owner === null || owner === undefined ? TEAM_TOKEN : (tokens.get(owner) ?? TEAM_TOKEN), byLevel }];
    });
  }, [heat, calibration, pickedKey, map, side, box, boxLevel, multiLevel, tokens]);

  const read = overlayData?.overlay ?? null;
  const step = read?.step_seconds ?? 0;
  const inBox = useMemo(() => {
    const rounds = read?.rounds ?? [];
    return box === null ? rounds : roundsThroughBox({ rounds, step, players: picked, box, tramo: WHOLE_ROUND });
  }, [read, step, picked, box]);
  const allMoves = useMemo(() => firstMovements({ rounds: inBox, step, players: picked, tramo: WHOLE_ROUND }), [inBox, step, picked]);
  const moves = useMemo(() => (isWholeRound(tramo) ? allMoves : firstMovements({ rounds: inBox, step, players: picked, tramo })), [allMoves, inBox, step, picked, tramo]);
  const histogram = useMemo(() => clockHistogram({ items: allMoves, secondsOf: (row) => row.seconds, keyOf: (row) => row.steamid64 }), [allMoves]);
  const histogramKeys = useMemo(
    (): HistogramKey[] =>
      [...new Set(allMoves.map((row) => row.steamid64))].map((id) => ({ key: id, label: names.get(id) ?? id, token: tokens.get(id) ?? '--fg-3' })),
    [allMoves, names, tokens],
  );
  const barcode = useMemo(() => allMoves.map((row) => ({ seconds: row.seconds, token: tokens.get(row.steamid64) ?? '--fg-3' })), [allMoves, tokens]);
  const refs = useMemo((): ReplayRef[] => {
    const seen = new Set<string>();
    const out: ReplayRef[] = [];
    for (const row of moves) {
      const key = `${row.matchId}:${row.round}`;
      if (seen.has(key)) continue;
      seen.add(key);
      out.push({ matchId: row.matchId, round: row.round });
    }
    return out;
  }, [moves]);
  const total = read?.rounds.length ?? 0;
  const capped = (read?.total ?? 0) > total;

  if (map === '') {
    return <EmptyState icon={MapIcon} title="Elige un mapa" description="El calor de los jugadores se dibuja sobre el radar de un mapa." />;
  }
  if (overlay.kind === 'failed' || heat.kind === 'failed') {
    return (
      <EmptyState
        icon={PlugZap}
        title="No se pudo leer el calor de los jugadores"
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  if (overlay.kind === 'loading') return <Skeleton role="status" aria-label="Leyendo las posiciones" className="aspect-square w-full max-w-[44rem]" />;
  if (geometry === null || calibration === null) {
    return <EmptyState icon={MapIcon} title="Sin radar para este mapa" description={`Ninguna partida de ${prettyMapName(map)} tiene un análisis con el que dibujar el radar.`} />;
  }

  const toggle = (id: string): void =>
    pick.onChange(picked.includes(id) ? picked.filter((entry) => entry !== id) : [...picked, id].slice(0, HEAT_MAX_PLAYERS));
  const title = `Jugadores en ${prettyMapName(map)}`;

  const parts: FinderParts = {
    panel: (
      <div className="flex flex-col gap-2.5">
        <div className="flex flex-col gap-1.5">
          <span className={CAPS_LABEL}>Jugadores</span>
          <div role="group" aria-label="Jugadores que cuenta" className="flex flex-wrap gap-1.5">
            {players.map((player) => (
              <Toggle
                key={player.steamid64}
                variant="filter"
                size="sm"
                pressed={picked.includes(player.steamid64)}
                disabled={!picked.includes(player.steamid64) && picked.length >= HEAT_MAX_PLAYERS}
                onPressedChange={() => toggle(player.steamid64)}
              >
                <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ background: `var(${tokens.get(player.steamid64) ?? '--fg-3'})` }} />
                {names.get(player.steamid64)}
              </Toggle>
            ))}
          </div>
          <p className="text-meta text-fg-3">{picked.length === 0 ? 'Sin selección cuenta todo el equipo.' : `Hasta ${HEAT_MAX_PLAYERS} jugadores, cada uno con su color.`}</p>
        </div>
        {multiLevel ? (
          <Segmented<RadarLevel | 'both'>
            label="Piso"
            value={floor}
            options={[{ value: 'both', label: 'Los dos' }, ...[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))]}
            onChange={(next) => {
              setFloor(next);
              setBox(null);
            }}
          />
        ) : null}
        <div className="flex flex-wrap items-center gap-2">
          {box === null ? (
            <span className="flex items-center gap-1.5 text-meta text-fg-2">
              Zona
              <InfoTip label="la zona" text="Arrastra sobre el radar para dibujar la zona; con zoom, Mayús + arrastrar." />
            </span>
          ) : (
            <Button type="button" variant="ghost" size="xs" onClick={() => setBox(null)}>
              <X aria-hidden />
              Quitar zona
            </Button>
          )}
        </div>
        <Button asChild variant="ghost" size="xs" className="self-start">
          <Link href={moreHeatHref}>
            <Flame aria-hidden />
            Muertes, bajas y utilidad
          </Link>
        </Button>
      </div>
    ),
    map: (
      <div className="flex flex-col gap-2">
        <PlayerHeatRadar
          geometry={geometry}
          levels={levels}
          multiLevel={multiLevel}
          heats={heats}
          box={box}
          boxLevel={boxLevel}
          onBox={(next) => {
            setBox(next === null ? null : next.box);
            if (next !== null) setBoxLevel(next.level);
          }}
        />
        {heat.kind === 'ready' ? <HeatNote heats={heats} boxed={box !== null} /> : null}
      </div>
    ),
    rounds: (
      <div className="flex flex-col gap-3">
        <ClockHistogram
          buckets={histogram.buckets}
          tallest={histogram.tallest}
          keys={histogramKeys}
          noun={{ one: 'jugador se mueve', many: 'jugadores se mueven' }}
          label="Primer movimiento por tramo de 10 segundos de la ronda"
          onPick={(bucket) => setTramo({ from: bucket.start, to: bucket.end })}
        />
        <h3 className="text-body-sm font-semibold text-fg-1 tabular-nums">
          {refs.length} de {total} {total === 1 ? 'ronda' : 'rondas'}
          <span className="ml-2 font-normal text-fg-2">con el primer movimiento en el tramo</span>
        </h3>
        {capped ? <p className="text-body-sm text-fg-2">La lista lee las {total} rondas más recientes de las {read?.total}; el calor cuenta todas.</p> : null}
        <MovementList rows={moves} names={names} tokens={tokens} matchLabel={matchLabel} />
      </div>
    ),
    footer: (
      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <Button asChild variant="outline" size="sm" className={refs.length === 0 ? 'pointer-events-none opacity-50' : undefined}>
            <Link href={findingHref({ refs, title, focus: picked, tramo })} aria-disabled={refs.length === 0}>
              <Layers aria-hidden />
              Reproducir {refs.length}/{total} rondas
            </Link>
          </Button>
        </div>
        <TramoBar tramo={tramo} onChange={setTramo} events={barcode} />
      </div>
    ),
    count: { matched: refs.length, total },
  };
  return <FramedParts frame={frame} parts={parts} />;
}

/** Under the map: what the empty heat means, the scale of a single series, or what several colours stand for. */
function HeatNote({ heats, boxed }: { heats: readonly PlayerHeat[]; boxed: boolean }): ReactNode {
  if (!heats.some((entry) => entry.byLevel.size > 0)) {
    return (
      <p className="text-body-sm text-fg-2">
        {boxed
          ? 'Nadie pasa por esa zona en este tramo. Ensancha el tramo o quita la zona.'
          : 'Ningún jugador vivo en este tramo, o sin posiciones guardadas en estas rondas. Ensancha el tramo o cambia los filtros.'}
      </p>
    );
  }
  if (heats.length === 1) return <HeatLegend kind="position" maxSeconds={0} />;
  return <p className="text-meta text-fg-3">Cada color es un jugador; cuanto más opaco, más tiempo pasó en la zona.</p>;
}
