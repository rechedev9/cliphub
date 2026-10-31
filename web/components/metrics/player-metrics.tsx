'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { PlugZap } from 'lucide-react';
import { getLibraryPlayerMetrics, type PlayerMetricsQuery } from '@/lib/api/library';
import type { MetricsReport } from '@/lib/api/metrics';
import type { ReportSide, ScoutMatchSummary } from '@/lib/api/scout';
import { matchHref } from '@/lib/clips/routes';
import { prettyMapName } from '@/lib/format';
import { ecoText, ppText, roleText } from '@/lib/metrics/advanced';
import { EmptyState } from '@/components/records/empty-state';
import { Segmented } from '@/components/tactical/spatial/spatial-controls';
import { Button } from '@/components/ui/button';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { MetricsView } from './metrics-view';

const ALL_MAPS = 'todos';
const LIMITS = ['10', '20', '50'] as const;
type Limit = (typeof LIMITS)[number];
const SIDES: readonly { value: ReportSide; label: string }[] = [
  { value: '', label: 'Ambos lados' },
  { value: 'CT', label: 'CT' },
  { value: 'T', label: 'T' },
];

type State = { kind: 'loading' } | { kind: 'ready'; report: MetricsReport } | { kind: 'offline' };

function Facts({ report }: { report: MetricsReport }): ReactNode {
  const p = report.players[0];
  if (!p) return null;
  const facts = [
    ['WPA', ppText(p.swing.wpa)],
    ['WPA por ronda', ppText(p.swing.per_round)],
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

function PlayerMetricsBody({ report, matches }: { report: MetricsReport; matches: ReadonlyMap<string, ScoutMatchSummary> }): ReactNode {
  if (report.players.length === 0) {
    return (
      <p className="text-body-sm text-fg-2">
        Ninguna partida analizada con este filtro. {report.matches > 0 ? `Se leyeron ${report.matches} partidas sin rondas en ese lado.` : ''}
      </p>
    );
  }
  return <MetricsView report={report} options={{ byMatch: true, matchHref }} matches={matches} lead={<Facts report={report} />} />;
}

/** A player's advanced metrics over their most recent matches, by map and side. */
export function PlayerMetrics({
  steamId,
  maps,
  matches,
}: {
  steamId: string;
  maps: readonly string[];
  matches: ReadonlyMap<string, ScoutMatchSummary>;
}): ReactNode {
  const [map, setMap] = useState(ALL_MAPS);
  const [side, setSide] = useState<ReportSide>('');
  const [limit, setLimit] = useState<Limit>('20');
  const [state, setState] = useState<State>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    const query: PlayerMetricsQuery = { side, limit: Number(limit) };
    if (map !== ALL_MAPS) query.map = map;
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
  }, [steamId, map, side, limit, attempt]);

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-wrap items-center gap-2">
        <Select value={map} onValueChange={setMap}>
          <SelectTrigger aria-label="Mapa" className="h-9 min-w-36">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={ALL_MAPS}>Todos los mapas</SelectItem>
            {maps.map((name) => (
              <SelectItem key={name} value={name}>
                {prettyMapName(name)}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Segmented label="Lado" value={side} options={SIDES} onChange={setSide} />
        <Segmented
          label="Partidas leídas"
          value={limit}
          options={LIMITS.map((value) => ({ value, label: `Últimas ${value}` }))}
          onChange={setLimit}
        />
      </div>
      {state.kind === 'loading' ? (
        <div role="status" aria-label="Calculando métricas" className="flex flex-col gap-3">
          <Skeleton className="h-16 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : null}
      {state.kind === 'offline' ? (
        <EmptyState
          icon={PlugZap}
          title="No se pudieron calcular las métricas"
          description="Comprueba que ClipHub Studio sigue abierto y vuelve a intentarlo."
          actions={
            <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
              Reintentar
            </Button>
          }
        />
      ) : null}
      {state.kind === 'ready' ? <PlayerMetricsBody report={state.report} matches={matches} /> : null}
    </div>
  );
}
