'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Info } from 'lucide-react';
import { LIBRARY_ROLES, getTeamSideStats } from '@/lib/api/library';
import type { ReportSide } from '@/lib/api/scout';
import type { SideStat, SideStats, SideStatsPlayer, TeamSideStats as TeamSideStatsReport } from '@/lib/api/side-stats';
import type { DeltaTone } from '@/lib/analysis/compare';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import {
  RATING_FORMULA_TEXT,
  SIDE_STAT_TILES,
  formatSideValue,
  groupMeanText,
  sideDelta,
  type SideStatTile,
} from '@/lib/analysis/side-stats';
import { matchHref } from '@/lib/clips/routes';
import { formatDecimal, formatShortDate, prettyMapName } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { EmptySample } from '@/components/analysis/empty-sample';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { FOCUS_RING } from '@/components/ui/button';
import { DELTA_TONE_CLASS } from '@/components/ui/delta-value';
import { DisclosureSummary } from '@/components/ui/disclosure-summary';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

const BORDER: Record<DeltaTone, string> = { better: 'border-l-outcome-win', worse: 'border-l-outcome-loss', even: 'border-l-border' };
const SIDE_TILES = SIDE_STAT_TILES.filter((tile) => tile.key !== 'maps_won');

function InfoTip({ label, text }: { label: string; text: string }): ReactNode {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button type="button" aria-label={`Cómo se calcula: ${label}`} className={cn('inline-flex rounded-sm text-fg-3 hover:text-fg-1', FOCUS_RING)}>
          <Info aria-hidden className="size-3.5" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="top" className="max-w-xs">
        {text}
      </TooltipContent>
    </Tooltip>
  );
}

function StatTile({ tile, stat }: { tile: SideStatTile; stat: SideStat }): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <div className={cn('flex min-w-0 flex-col gap-1 rounded-md border-l-4 bg-surface-2 px-3 py-2', BORDER[delta?.tone ?? 'even'])}>
      <dt className="flex items-center gap-1 text-meta leading-tight text-fg-3">
        {tile.label}
        {tile.formula ? <InfoTip label={tile.label} text={tile.formula} /> : null}
      </dt>
      <dd className="flex flex-col gap-0.5">
        <span className="text-title font-semibold text-fg-1 tabular-nums">{formatSideValue(stat.value, tile.unit)}</span>
        {delta ? <span className={cn('text-meta font-medium tabular-nums', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
        <span className="text-meta text-fg-3 tabular-nums">{groupMeanText(stat, tile.unit)}</span>
      </dd>
    </div>
  );
}

function SideCell({ tile, stat }: { tile: SideStatTile; stat: SideStat }): ReactNode {
  const delta = sideDelta(stat, tile.unit);
  return (
    <td className="px-2 py-1.5 text-end tabular-nums">
      <span className="font-medium text-fg-1">{formatSideValue(stat.value, tile.unit)}</span>
      {delta ? <span className={cn('ml-2 text-meta', DELTA_TONE_CLASS[delta.tone])}>{delta.text}</span> : null}
      <span className="block text-meta text-fg-3">{groupMeanText(stat, tile.unit)}</span>
    </td>
  );
}

function SidesTable({ ct, t }: { ct: SideStats; t: SideStats }): ReactNode {
  return (
    <section aria-labelledby="side-stats-sides" className="studio-panel flex flex-col gap-2 p-3">
      <h2 id="side-stats-sides" className="text-body font-semibold text-fg-1">
        Por lado
      </h2>
      <table className="w-full text-body-sm">
        <thead>
          <tr className="border-b border-border-subtle text-meta text-fg-3">
            <th scope="col" className="px-2 py-1.5 text-start font-normal">
              Cifra
            </th>
            <th scope="col" className="px-2 py-1.5 text-end font-semibold text-side-ct">
              CT
            </th>
            <th scope="col" className="px-2 py-1.5 text-end font-semibold text-side-t">
              T
            </th>
          </tr>
        </thead>
        <tbody>
          {SIDE_TILES.map((tile) => (
            <tr key={tile.key} className="border-b border-border-subtle last:border-0">
              <th scope="row" className="px-2 py-1.5 text-start font-normal text-fg-2">
                {tile.label}
              </th>
              <SideCell tile={tile} stat={ct[tile.key]} />
              <SideCell tile={tile} stat={t[tile.key]} />
            </tr>
          ))}
        </tbody>
      </table>
    </section>
  );
}

function SideLabel({ side }: { side: ReportSide }): ReactNode {
  if (side === 'CT') return <span className="font-semibold text-side-ct">CT</span>;
  if (side === 'T') return <span className="font-semibold text-side-t">T</span>;
  return (
    <span className="font-semibold">
      <span className="text-side-ct">CT</span> y <span className="text-side-t">T</span>
    </span>
  );
}

function roleText(role: string): string {
  const known = LIBRARY_ROLES.find((entry) => entry === role);
  return known === undefined ? ROLE_LABEL.unknown : ROLE_LABEL[known];
}

function PlayerCard({ player, side }: { player: SideStatsPlayer; side: ReportSide }): ReactNode {
  const name = player.name || player.steamid64;
  const lines = [
    { label: 'K/D', value: formatDecimal(player.kd, 2) },
    { label: 'ADR', value: formatDecimal(player.adr, 1) },
    { label: 'KPR', value: formatDecimal(player.kpr, 2) },
    { label: 'DPR', value: formatDecimal(player.dpr, 2) },
  ];
  return (
    <li className="studio-panel flex min-w-0 flex-col gap-2 p-3">
      <div className="flex min-w-0 items-center gap-2">
        <PlayerAvatar nickname={name} playerID={player.steamid64} size={32} />
        <div className="min-w-0">
          <p className="truncate text-body font-semibold text-fg-1">{name}</p>
          <p className="text-meta text-fg-2">
            <SideLabel side={side} /> · {roleText(player.role)}
          </p>
        </div>
      </div>
      <p className="flex items-baseline gap-1.5">
        <span className="text-title font-semibold text-fg-1 tabular-nums">{formatDecimal(player.rating, 2)}</span>
        <span className="text-meta text-fg-3">Rating</span>
        <InfoTip label="Rating" text={RATING_FORMULA_TEXT} />
      </p>
      <dl className="grid grid-cols-4 gap-2">
        {lines.map((line) => (
          <div key={line.label} className="flex flex-col">
            <dt className="text-meta text-fg-3">{line.label}</dt>
            <dd className="text-body-sm font-medium text-fg-1 tabular-nums">{line.value}</dd>
          </div>
        ))}
      </dl>
      <p className="text-meta text-fg-3 tabular-nums">{player.rounds === 1 ? '1 ronda' : `${player.rounds} rondas`}</p>
    </li>
  );
}

function groupText(report: TeamSideStatsReport): string {
  if (report.group_teams === 0) return 'Ningún equipo del grupo tiene rondas con este mapa, lado, fechas y compras, así que no hay media.';
  if (report.compare === 'teams') return `Comparado con ${report.compare_teams.map((team) => team.name).join(', ')}, cada uno con el mismo peso.`;
  const teams = report.group_teams === 1 ? '1 equipo' : `${report.group_teams} equipos`;
  return `Comparado con el resto de la biblioteca: ${teams} con rondas en este alcance, cada uno con el mismo peso.`;
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

/** Estadísticas: the team's figures in the scope against the "Comparar con" group, and a card per player. */
export function TeamSideStats({ teamId, scope, lineup, compare, onCounts }: TeamSideStatsProps): ReactNode {
  const [data, setData] = useState<TeamSideStatsReport | null | 'failed'>(null);
  const [now] = useState(() => Date.now());
  const { map, side, opponent, buy, opponentBuy } = scope;
  const { from, to } = scopeDateRange(scope.period, now);
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
      buy,
      opponent_buy: opponentBuy,
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
  }, [teamId, map, side, from, to, opponent, lineup, buy, opponentBuy, compareKey, onCounts]);

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
    <div className="flex flex-col gap-4">
      <section aria-labelledby="side-stats-overall" className="flex flex-col gap-2">
        <h2 id="side-stats-overall" className="text-body font-semibold text-fg-1">
          Estadísticas generales
        </h2>
        <p className="measure-read text-body-sm text-fg-2">{groupText(data)}</p>
        <dl className="grid grid-cols-[repeat(auto-fill,minmax(10rem,1fr))] gap-2">
          {SIDE_STAT_TILES.map((tile) => (
            <StatTile key={tile.key} tile={tile} stat={data.overall[tile.key]} />
          ))}
        </dl>
      </section>
      {side === '' ? <SidesTable ct={data.ct} t={data.t} /> : null}
      <section aria-labelledby="side-stats-players" className="flex flex-col gap-2">
        <h2 id="side-stats-players" className="text-body font-semibold text-fg-1">
          Rendimiento de jugadores
        </h2>
        <ul className="grid grid-cols-[repeat(auto-fill,minmax(14rem,1fr))] gap-2">
          {data.players.map((player) => (
            <PlayerCard key={player.steamid64} player={player} side={side} />
          ))}
        </ul>
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
