'use client';

import type { ReactNode } from 'react';
import type { SideStat, TeamSideStats as TeamSideStatsReport } from '@/lib/api/side-stats';
import type { DeltaTone } from '@/lib/analysis/compare';
import { SIDE_STAT_TILES, formatSideValue, groupMeanText, sideDelta, type SideStatTile } from '@/lib/analysis/side-stats';
import { EmptySample } from '@/components/analysis/empty-sample';
import { InfoTip } from '@/components/analysis/info-tip';
import { DELTA_TONE_CLASS } from '@/components/ui/delta-value';
import { SlashTitle } from '@/components/records/document-page';
import { EdgeTile, EdgeTileGrid, type EdgeTone } from '@/components/records/edge-tile';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { PlayerStatCard } from './player-stat-card';
import { TeamSidesCard } from './team-sides-card';
import { useTeamSideStats, type TeamSideStatsQuery } from './use-team-side-stats';

const TONE: Record<DeltaTone, EdgeTone> = { better: 'good', worse: 'bad', even: 'neutral' };

function StatTile({ tile, stat }: { tile: SideStatTile; stat: SideStat }): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <EdgeTile
      label={tile.label}
      wrapLabel
      info={tile.formula ? <InfoTip label={tile.label} text={tile.formula} /> : null}
      tone={TONE[delta?.tone ?? 'even']}
      value={formatSideValue(stat.value, tile.unit)}
      note={
        tile.unit === 'count' ? null : (
          <>
            {delta ? <span className={cn('font-medium', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
            {delta ? ' · ' : null}
            {groupMeanText(stat, tile.unit)}
          </>
        )
      }
    />
  );
}

/** The InfoTip text that says which teams the means come from. */
export function groupText(report: TeamSideStatsReport): string {
  if (report.group_teams === 0) return 'Ningún equipo del grupo tiene rondas con este mapa, lado, fechas y compras, así que no hay media.';
  const who =
    report.compare === 'teams'
      ? `Comparado con ${report.compare_teams.map((team) => team.name).join(', ')}`
      : `Comparado con el resto de la biblioteca (${report.group_teams === 1 ? '1 equipo' : `${report.group_teams} equipos`} con rondas en este alcance)`;
  const matches = report.group_matches === 1 ? '1 partida' : `${report.group_matches} partidas`;
  const cap = report.group_capped ? ` De cada equipo cuentan sus 20 partidas más recientes.` : '';
  return `${who}, sobre ${matches}. Cada equipo pesa lo mismo y entra en una cifra con al menos ${report.group_min_rounds} rondas detrás.${cap}`;
}

/** The loading, failed and empty states the Resumen and Rendimiento tabs share; null once there is data. */
export function sideStatsState(data: TeamSideStatsReport | null | 'failed'): ReactNode {
  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron leer las estadísticas de este equipo. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Leyendo las estadísticas" className="flex flex-col gap-3">
        <Skeleton className="h-40 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }
  if (data.rounds === 0) {
    return (
      <EmptySample>
        <p className="text-body-sm text-fg-2">Sin rondas analizadas con este filtro. Amplía las fechas o quita el mapa, el rival o las compras.</p>
      </EmptySample>
    );
  }
  return null;
}

/** Resumen, EDGE's Overview: the Lados banner, a card per player and the overall tiles against the "Comparar con" group. */
export function TeamSideStats(props: TeamSideStatsQuery): ReactNode {
  const data = useTeamSideStats(props);
  const state = sideStatsState(data);
  if (state !== null || data === null || data === 'failed') return state;
  const { side, map } = props.scope;
  return (
    <div className="flex flex-col gap-6">
      <section aria-label="Lados" className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <SlashTitle>Lados</SlashTitle>
          <InfoTip label="la media del grupo" text={groupText(data)} />
        </div>
        <TeamSidesCard ct={data.ct} t={data.t} side={side} map={map} />
      </section>
      <section aria-label="Jugadores" className="flex flex-col gap-2">
        <SlashTitle>Jugadores</SlashTitle>
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(8.5rem,1fr))] gap-3">
          {data.players.map((player) => (
            <PlayerStatCard key={player.steamid64} player={player} side={side} />
          ))}
        </ul>
      </section>
      <section aria-label="Estadísticas generales" className="flex flex-col gap-2">
        <SlashTitle>Estadísticas generales</SlashTitle>
        <EdgeTileGrid label="Estadísticas generales" className="grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5">
          {SIDE_STAT_TILES.map((tile) => (
            <StatTile key={tile.key} tile={tile} stat={data.overall[tile.key]} />
          ))}
        </EdgeTileGrid>
      </section>
      {data.skipped.length > 0 ? (
        <p className="text-body-sm text-fg-2">
          {data.skipped.length === 1 ? 'Una partida sin análisis no cuenta.' : `${data.skipped.length} partidas sin análisis no cuentan.`}
        </p>
      ) : null}
    </div>
  );
}
