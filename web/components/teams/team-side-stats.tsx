'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getTeamSideStats } from '@/lib/api/library';
import type { SideStat, TeamSideStats as TeamSideStatsReport } from '@/lib/api/side-stats';
import type { DeltaTone } from '@/lib/analysis/compare';
import { scopeDateRange, type AnalysisScope, type ScopeField } from '@/lib/analysis/scope';
import { teamScopeEntries } from '@/lib/analysis/team-scope';
import {
  SIDE_STAT_TILES,
  formatSideValue,
  groupMeanText,
  sideDelta,
  type SideStatTile,
} from '@/lib/analysis/side-stats';
import { matchHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { EmptySample } from '@/components/analysis/empty-sample';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { InfoTip } from '@/components/analysis/info-tip';
import { FOCUS_RING } from '@/components/ui/button';
import { DELTA_TONE_CLASS } from '@/components/ui/delta-value';
import { SlashTitle } from '@/components/records/document-page';
import { EdgeTile, EdgeTileGrid, type EdgeTone } from '@/components/records/edge-tile';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { PlayerStatCard } from './player-stat-card';
import { TeamSidesCard } from './team-sides-card';

const TONE: Record<DeltaTone, EdgeTone> = { better: 'good', worse: 'bad', even: 'neutral' };

function StatTile({ tile, stat }: { tile: SideStatTile; stat: SideStat }): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <EdgeTile
      label={tile.label}
      info={tile.formula ? <InfoTip label={tile.label} text={tile.formula} /> : null}
      tone={TONE[delta?.tone ?? 'even']}
      value={formatSideValue(stat.value, tile.unit)}
      note={delta ? <span className={cn('font-medium', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
      detail={groupMeanText(stat, tile.unit)}
    />
  );
}

function groupText(report: TeamSideStatsReport): string {
  if (report.group_teams === 0) return 'Ningún equipo del grupo tiene rondas con este mapa, lado, fechas y compras, así que no hay media.';
  const who =
    report.compare === 'teams'
      ? `Comparado con ${report.compare_teams.map((team) => team.name).join(', ')}`
      : `Comparado con el resto de la biblioteca (${report.group_teams === 1 ? '1 equipo' : `${report.group_teams} equipos`} con rondas en este alcance)`;
  const matches = report.group_matches === 1 ? '1 partida' : `${report.group_matches} partidas`;
  const cap = report.group_capped ? ` De cada equipo cuentan sus 20 partidas más recientes.` : '';
  return `${who}, sobre ${matches}. Cada equipo pesa lo mismo y entra en una cifra con al menos ${report.group_min_rounds} rondas detrás.${cap}`;
}

function MatchList({ report }: { report: TeamSideStatsReport }): ReactNode {
  const count = report.match_list.length;
  return (
    <details className="text-body-sm">
      <DisclosureSummary className="min-h-10 text-fg-2">{count === 1 ? 'Ver la partida' : `Ver las ${count} partidas`}</DisclosureSummary>
      <ul className="flex flex-col divide-y divide-border-subtle">
        {report.match_list.map((match) => (
          <li key={match.match_id}>
            <Link href={matchHref(match.match_id)} className={cn('flex flex-wrap items-baseline gap-x-3 rounded-sm py-1.5 hover:text-fg-1', FOCUS_RING)}>
              <span className="text-fg-3 tabular-nums">{formatShortDate(match.played_at)}</span>
              <span className="text-fg-1">{prettyMapName(match.map)}</span>
              <span className="text-fg-2">{match.opponent || 'Rival sin nombre'}</span>
              <span className={match.won ? 'text-outcome-win' : 'text-outcome-loss'}>{match.won ? 'Victoria' : 'Sin victoria'}</span>
              <span className="text-meta text-fg-3 tabular-nums">{match.rounds === 1 ? '1 ronda' : `${match.rounds} rondas`}</span>
            </Link>
          </li>
        ))}
      </ul>
    </details>
  );
}

export type TeamSideStatsProps = {
  teamId: string;
  scope: AnalysisScope;
  lineup: readonly string[];
  /** "Comparar con" team ids; empty is the whole library. */
  compare: readonly string[];
  onCounts: (counts: ScopeCounts | null) => void;
};

/** The scope fields Estadísticas sends beyond map, side, period, rival and lineup. */
const SIDE_STATS_CRITERIA: readonly ScopeField[] = ['buy', 'opponentBuy', 'excludeTactics', 'matchTags', 'excludeMatchTags', 'events'];

/** Estadísticas: the team's figures in the scope against the "Comparar con" group, and a card per player. */
export function TeamSideStats({ teamId, scope, lineup, compare, onCounts }: TeamSideStatsProps): ReactNode {
  const [data, setData] = useState<TeamSideStatsReport | null | 'failed'>(null);
  const [now] = useState(() => Date.now());
  const { map, side, opponent } = scope;
  const { from, to } = scopeDateRange(scope.period, now);
  const criteria = useMemo(() => teamScopeEntries(scope, SIDE_STATS_CRITERIA), [scope]);
  const compareKey = compare.join(',');

  useEffect(() => {
    let active = true;
    setData(null);
    onCounts(null);
    getTeamSideStats(teamId, {
      maps: map === '' ? [] : [map],
      side,
      from,
      to,
      opponent_ids: opponent === '' ? [] : [opponent],
      lineup,
      criteria,
      compare: compareKey === '' ? [] : compareKey.split(','),
    })
      .then((report) => {
        if (!active) return;
        setData(report);
        onCounts({ maps: report.matches, rounds: report.rounds });
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, map, side, from, to, opponent, lineup, criteria, compareKey, onCounts]);

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
  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2">
          <SlashTitle>Lados</SlashTitle>
          {data.group_teams === 0 ? null : <InfoTip label="la media del grupo" text={groupText(data)} />}
        </div>
        <TeamSidesCard ct={data.ct} t={data.t} overall={data.overall} side={side} map={map} />
        {data.group_teams === 0 ? <p className="measure-read text-body-sm text-fg-2">{groupText(data)}</p> : null}
      </div>
      <section aria-label="Jugadores" className="flex flex-col gap-2">
        <SlashTitle>Jugadores</SlashTitle>
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(8.5rem,1fr))] gap-2">
          {data.players.map((player) => (
            <PlayerStatCard key={player.steamid64} player={player} side={side} />
          ))}
        </ul>
      </section>
      <section aria-label="Estadísticas generales" className="flex flex-col gap-2">
        <SlashTitle>Estadísticas generales</SlashTitle>
        <EdgeTileGrid label="Estadísticas generales" className="grid-cols-[repeat(auto-fill,minmax(10.5rem,1fr))]">
          {SIDE_STAT_TILES.map((tile) => (
            <StatTile key={tile.key} tile={tile} stat={data.overall[tile.key]} />
          ))}
        </EdgeTileGrid>
      </section>
      <MatchList report={data} />
      {data.skipped.length > 0 ? (
        <p className="text-body-sm text-fg-2">
          {data.skipped.length === 1 ? 'Una partida sin análisis no cuenta.' : `${data.skipped.length} partidas sin análisis no cuentan.`}
        </p>
      ) : null}
    </div>
  );
}
