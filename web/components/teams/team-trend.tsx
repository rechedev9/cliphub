'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { getLibraryMatch, type LibraryMatch } from '@/lib/api/library';
import type { BaselineMetric } from '@/lib/api/baseline';
import { METRIC_INFO, baselineValue } from '@/lib/analysis/reference';
import { TEAM_TREND_LABEL, TEAM_TREND_METRICS, teamTrend, type TeamTrendMetric } from '@/lib/analysis/team-trend';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { LineChart } from '@/components/ui/line-chart';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { useReferences } from '@/hooks/use-references';

/** The chart reads the latest matches; older ones would squeeze the labels. */
const MAX_MATCHES = 12;
const MAX_PLAYERS = 5;
const PLAYER_COLORS = ['--chart-1', '--chart-2', '--chart-3', '--chart-6', '--chart-7'];
const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const FORMAT: Readonly<Record<TeamTrendMetric, (value: number) => string>> = {
  kpr: METRIC_INFO.kpr.format,
  adr: METRIC_INFO.adr.format,
  kd: (value) => decimal2.format(value),
};
const BASELINE: Readonly<Record<TeamTrendMetric, BaselineMetric | null>> = { kpr: 'kpr', adr: 'adr', kd: null };

/** The team's latest matches, one line per regular player plus the team's own line and the library mean. */
export function TeamTrend({ teamId, matches, map }: { teamId: string; matches: readonly LibraryMatch[]; map: string }): ReactNode {
  const [details, setDetails] = useState<LibraryMatch[] | null | 'failed'>(null);
  const [metric, setMetric] = useState<TeamTrendMetric>('kpr');
  const references = useReferences(map);
  const ids = [...matches]
    .sort((a, b) => Date.parse(b.played_at) - Date.parse(a.played_at))
    .slice(0, MAX_MATCHES)
    .map((match) => match.id)
    .join(',');

  useEffect(() => {
    let active = true;
    setDetails(null);
    Promise.all((ids === '' ? [] : ids.split(',')).map((id) => getLibraryMatch(id)))
      .then((list) => {
        if (active) setDetails(list);
      })
      .catch(() => {
        if (active) setDetails('failed');
      });
    return () => {
      active = false;
    };
  }, [ids]);

  let body: ReactNode;
  if (details === 'failed') {
    body = <p role="alert" className="text-body-sm text-destructive">No se pudieron leer las partidas del equipo.</p>;
  } else if (details === null) {
    body = <Skeleton className="h-56 w-full" />;
  } else if (details.length < 2) {
    body = <p className="text-body-sm text-fg-2">Hacen falta al menos dos partidas con este filtro para dibujar la evolución.</p>;
  } else {
    const trend = teamTrend(details, teamId, { metric, minMatches: Math.min(2, details.length) });
    const baseline = BASELINE[metric];
    const mean = baseline !== null && references !== null && references !== 'failed' ? baselineValue(references.library, baseline) : null;
    body = (
      <LineChart
        label={`${TEAM_TREND_LABEL[metric]} por partida y jugador, la más antigua primero`}
        categories={trend.matches.map((match) => ({
          label: `${prettyMapName(match.map)} ${formatShortDate(match.played_at)}`,
          detail: `${match.round_count} rondas`,
        }))}
        series={[
          { id: 'team', label: 'Equipo', values: trend.team, color: '--fg-1', emphasis: true },
          ...trend.players.slice(0, MAX_PLAYERS).map((player, index) => ({
            id: player.id,
            label: player.label,
            values: player.values,
            color: PLAYER_COLORS[index % PLAYER_COLORS.length],
          })),
        ]}
        format={FORMAT[metric]}
        reference={mean === null ? undefined : { value: mean, label: 'Biblioteca' }}
        toolbar={
          <Segmented
            label="Cifra del gráfico"
            value={metric}
            options={TEAM_TREND_METRICS.map((value) => ({ value, label: TEAM_TREND_LABEL[value] }))}
            onChange={setMetric}
          />
        }
      />
    );
  }

  return (
    <section aria-label="Rendimiento reciente" className="studio-panel flex flex-col gap-2 px-3 py-2.5">
      <h2 className="text-body-sm font-semibold text-fg-1">Rendimiento reciente</h2>
      {body}
    </section>
  );
}
