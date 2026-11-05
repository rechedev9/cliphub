'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { PlugZap } from 'lucide-react';
import { getLibraryPlayerMetrics, type LibraryMatch, type PlayerMetricsQuery } from '@/lib/api/library';
import type { MetricsReport, PlayerMetrics as PlayerMetricsLine } from '@/lib/api/metrics';
import type { ScoutMatchSummary } from '@/lib/api/scout';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { matchHref } from '@/lib/clips/routes';
import { ecoText, ppText, roleText } from '@/lib/metrics/advanced';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { EmptyState } from '@/components/records/empty-state';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { StatCard, StatCardGroup } from '@/components/ui/stat-card';
import { MetricsView } from './metrics-view';

/** The matches read when the URL names no limit; the API's own default. */
export const PLAYER_METRICS_LIMIT = 20;
export const PLAYER_METRICS_LIMITS = [10, 20, 50] as const;

const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const decimal1 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const two = (value: number): string => decimal2.format(value);
const one = (value: number): string => decimal1.format(value);
const COMPARISON_LABEL = 'Media del resto de jugadores de estas partidas';

type State = { kind: 'loading' } | { kind: 'ready'; report: MetricsReport } | { kind: 'offline' };

type Totals = { kills: number; deaths: number; damage: number; rounds: number };

const kd = (t: Pick<Totals, 'kills' | 'deaths'>): number | null => (t.deaths > 0 ? t.kills / t.deaths : null);
const perRound = (value: number, rounds: number): number | null => (rounds > 0 ? value / rounds : null);

/**
 * Everyone else's lines in the matches the report read, from the match details
 * the record already loaded. Lines cover the whole match, so they only compare
 * with a report that reads both sides.
 */
function othersTotals(player: PlayerMetricsLine, details: ReadonlyMap<string, LibraryMatch>): Totals | null {
  const totals: Totals = { kills: 0, deaths: 0, damage: 0, rounds: 0 };
  for (const line of player.match_lines) {
    for (const other of details.get(line.match_id)?.players ?? []) {
      if (other.steamid64 === player.steamid64) continue;
      totals.kills += other.kills;
      totals.deaths += other.deaths;
      totals.damage += other.damage;
      totals.rounds += other.rounds;
    }
  }
  return totals.rounds > 0 ? totals : null;
}

/** The player's own damage per match from the same details, oldest first; ADR is not in the metrics. */
function ownDamage(player: PlayerMetricsLine, details: ReadonlyMap<string, LibraryMatch>): { adr: number | null; trend: number[] } {
  let damage = 0;
  let rounds = 0;
  const trend: number[] = [];
  for (const line of [...player.match_lines].reverse()) {
    const own = details.get(line.match_id)?.players?.find((p) => p.steamid64 === player.steamid64);
    if (own === undefined || own.rounds === 0) continue;
    damage += own.damage;
    rounds += own.rounds;
    trend.push(own.damage / own.rounds);
  }
  return { adr: perRound(damage, rounds), trend };
}

function Headline({ report, details, bothSides }: { report: MetricsReport; details: ReadonlyMap<string, LibraryMatch>; bothSides: boolean }): ReactNode {
  const p = report.players[0];
  if (!p) return null;
  const lines = [...p.match_lines].reverse();
  const others = bothSides ? othersTotals(p, details) : null;
  const damage = ownDamage(p, details);
  const compare = (value: number | null) => (others === null ? undefined : { value, label: COMPARISON_LABEL });
  const facts = [
    ['Bajas ajustadas', `${ecoText(p.eco.kills)} de ${p.kills}`],
    ['Muertes ajustadas', `${ecoText(p.eco.deaths)} de ${p.deaths}`],
    ['Trades hechos', String(p.trades.trade_kills.count)],
    ['Asistencias de flash', p.utility.measured ? String(p.utility.flash.assists.count) : 'Sin medir'],
    ['Rol en estas partidas', roleText(p.role)],
  ];
  return (
    <div className="flex flex-col gap-4">
      <StatCardGroup label="Cifras principales">
        <StatCard
          label="K/D"
          value={kd(p)}
          format={two}
          comparison={compare(others && kd(others))}
          epsilon={0.005}
          trend={lines.map((l) => (l.deaths > 0 ? l.kills / l.deaths : l.kills))}
          sample={`${p.kills} bajas, ${p.deaths} muertes`}
        />
        <StatCard
          label="Bajas por ronda"
          value={perRound(p.kills, p.rounds)}
          format={two}
          comparison={compare(others && perRound(others.kills, others.rounds))}
          epsilon={0.005}
          trend={lines.map((l) => (l.rounds > 0 ? l.kills / l.rounds : 0))}
          sample={`${p.rounds} rondas`}
        />
        {bothSides ? (
          <StatCard
            label="ADR"
            value={damage.adr}
            format={one}
            comparison={compare(others && perRound(others.damage, others.rounds))}
            epsilon={0.05}
            trend={damage.trend}
          />
        ) : null}
        <StatCard
          label="WPA por ronda"
          value={p.swing.per_round}
          format={(value) => ppText(value)}
          formatDelta={(delta) => ppText(delta)}
          trend={lines.map((l) => (l.rounds > 0 ? l.wpa / l.rounds : 0))}
          sample={`${ppText(p.swing.wpa)} en total`}
        />
      </StatCardGroup>
      <dl className="flex flex-wrap gap-x-8 gap-y-3">
        {facts.map(([label, value]) => (
          <div key={label} className="flex flex-col gap-0.5">
            <dt className="text-meta text-fg-3">{label}</dt>
            <dd className="text-body-sm text-fg-1">{value}</dd>
          </div>
        ))}
      </dl>
    </div>
  );
}

function PlayerMetricsBody({
  report,
  matches,
  details,
  bothSides,
}: {
  report: MetricsReport;
  matches: ReadonlyMap<string, ScoutMatchSummary>;
  details: ReadonlyMap<string, LibraryMatch>;
  bothSides: boolean;
}): ReactNode {
  if (report.players.length === 0) {
    return (
      <p className="text-body-sm text-fg-2">
        Ninguna partida analizada con este filtro. {report.matches > 0 ? `Se leyeron ${report.matches} partidas sin rondas en ese lado.` : ''}
      </p>
    );
  }
  return (
    <MetricsView
      report={report}
      options={{ byMatch: true, matchHref }}
      matches={matches}
      lead={<Headline report={report} details={details} bothSides={bothSides} />}
    />
  );
}

/** A player's advanced metrics over their most recent matches, read through the page's analysis scope. */
export function PlayerMetrics({
  steamId,
  scope,
  matches,
  details,
  onCounts,
}: {
  steamId: string;
  scope: AnalysisScope;
  matches: ReadonlyMap<string, ScoutMatchSummary>;
  /** Full match details by id, with every player's line, for the comparison. */
  details: ReadonlyMap<string, LibraryMatch>;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [state, setState] = useState<State>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [now] = useState(() => Date.now());
  const { map, side } = scope;
  const limit = scope.limit ?? PLAYER_METRICS_LIMIT;
  const { from, to } = scopeDateRange(scope.period, now);

  useEffect(() => {
    let active = true;
    setState({ kind: 'loading' });
    onCounts(null);
    const query: PlayerMetricsQuery = { side, limit, from, to };
    if (map !== '') query.map = map;
    getLibraryPlayerMetrics(steamId, query)
      .then((report) => {
        if (!active) return;
        setState({ kind: 'ready', report });
        onCounts({ maps: report.matches, rounds: report.rounds });
      })
      .catch(() => {
        if (active) setState({ kind: 'offline' });
      });
    return () => {
      active = false;
    };
  }, [steamId, map, side, limit, from, to, attempt, onCounts]);

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
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        }
      />
    );
  }
  return <PlayerMetricsBody report={state.report} matches={matches} details={details} bothSides={side === ''} />;
}
