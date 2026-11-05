'use client';

import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { PlugZap } from 'lucide-react';
import { getLibraryPlayerMetrics, type PlayerMetricsQuery } from '@/lib/api/library';
import type { MetricsReport } from '@/lib/api/metrics';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { matchHref } from '@/lib/clips/routes';
import { ecoText, roleText } from '@/lib/metrics/advanced';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { MetricsView } from './metrics-view';

/** The matches read when the URL names no limit; the API's own default. */
export const PLAYER_METRICS_LIMIT = 20;
export const PLAYER_METRICS_LIMITS = [10, 20, 50] as const;

/** Sections the player page shows as their own tabs. */
const OWN_TABS = ['metricas-compra', 'metricas-mapas'] as const;

export type PlayerReportState = { kind: 'loading' } | { kind: 'ready'; report: MetricsReport } | { kind: 'offline' };

/** The player's metrics report for the page's scope, read once and shared by the tabs that read it. */
export function usePlayerReport(steamId: string, scope: AnalysisScope): { state: PlayerReportState; retry: () => void } {
  const [state, setState] = useState<PlayerReportState>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [now] = useState(() => Date.now());
  const { map, side } = scope;
  const limit = scope.limit ?? PLAYER_METRICS_LIMIT;
  const { from, to } = scopeDateRange(scope.period, now);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    const query: PlayerMetricsQuery = { side, limit, from, to };
    if (map !== '') query.map = map;
    getLibraryPlayerMetrics(steamId, query)
      .then((report) => {
        if (active) setState({ kind: 'ready', report });
      })
      .catch(() => {
        if (active) setState({ kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [steamId, map, side, limit, from, to, attempt]);

  const retry = useCallback(() => setAttempt((n) => n + 1), []);
  return { state, retry };
}

/** Loading and failure of the shared report; `children` renders once it is read. */
export function PlayerReportGate({
  state,
  retry,
  children,
}: {
  state: PlayerReportState;
  retry: () => void;
  children: (report: MetricsReport) => ReactNode;
}): ReactNode {
  if (state.kind === 'loading') {
    return (
      <div role="status" aria-label="Calculando métricas" className="flex flex-col gap-3">
        <Skeleton className="h-28 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  if (state.kind === 'offline') {
    return (
      <EmptyState
        icon={PlugZap}
        title="No se pudieron calcular las métricas"
        description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
        actions={
          <Button type="button" variant="outline" size="sm" onClick={retry}>
            Reintentar
          </Button>
        }
      />
    );
  }
  if (state.report.players.length === 0) {
    return (
      <p className="text-body-sm text-fg-2">
        Ninguna partida analizada con este filtro.{' '}
        {state.report.matches > 0 ? `Se leyeron ${state.report.matches} partidas sin rondas en ese lado.` : 'Amplía las fechas o quita el mapa.'}
      </p>
    );
  }
  return children(state.report);
}

function Facts({ report }: { report: MetricsReport }): ReactNode {
  const p = report.players[0];
  if (!p) return null;
  const facts = [
    ['Bajas ajustadas', `${ecoText(p.eco.kills)} de ${p.kills}`],
    ['Muertes ajustadas', `${ecoText(p.eco.deaths)} de ${p.deaths}`],
    ['Trades hechos', String(p.trades.trade_kills.count)],
    ['Asistencias de flash', p.utility.measured ? String(p.utility.flash.assists.count) : 'Sin medir'],
    ['Rol en estas partidas', roleText(p.role)],
  ];
  return (
    <dl className="flex flex-wrap gap-x-8 gap-y-3">
      {facts.map(([label, value]) => (
        <div key={label} className="flex flex-col gap-0.5">
          <dt className="text-meta text-fg-3">{label}</dt>
          <dd className="text-body-sm text-fg-1">{value}</dd>
        </div>
      ))}
    </dl>
  );
}

/** A player's advanced metrics, section by section; per buy and per map have their own tabs. */
export function PlayerMetrics({ report, matches }: { report: MetricsReport; matches: ReadonlyMap<string, ScoutMatchSummary> }): ReactNode {
  return (
    <MetricsView report={report} options={{ byMatch: true, matchHref }} matches={matches} lead={<Facts report={report} />} omit={OWN_TABS} />
  );
}
