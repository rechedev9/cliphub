'use client';

import { useState, type ReactNode } from 'react';
import type { LibraryMatch } from '@/lib/api/library';
import { BASELINE_METRICS, type Baseline, type BaselineMetric, type BaselineScope } from '@/lib/api/baseline';
import type { MetricsReport } from '@/lib/api/metrics';
import {
  METRIC_INFO,
  SCOPE_LABEL,
  baselineSampleText,
  baselineValue,
  metricReliability,
  minRoundsNote,
  playerEvidence,
  playerReferenceValues,
  proSourceText,
  referenceLabel,
} from '@/lib/analysis/reference';
import { TREND_BASELINE, TREND_LABEL, TREND_METRICS, trendPoints, type MatchDamage, type TrendMetric } from '@/lib/analysis/player-trend';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { dateText } from '@/lib/reports/scout-sections';
import { LineChart } from '@/components/ui/line-chart';
import { Segmented } from '@/components/ui/segmented';
import { SlashTitle } from '@/components/records/document-page';
import { EdgeTileGrid } from '@/components/records/edge-tile';
import { PlayerMetricTile } from '@/components/players/player-metric-tile';
import { useReferences } from '@/hooks/use-references';

/** Trend points need a few matches to say anything. */
const MIN_TREND_POINTS = 2;

const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const TREND_FORMAT: Readonly<Record<TrendMetric, (value: number) => string>> = {
  kd: (value) => decimal2.format(value),
  kpr: METRIC_INFO.kpr.format,
  adr: METRIC_INFO.adr.format,
  wpa: METRIC_INFO.wpa_per_round.format,
};

/** The player's own damage per match from the match details the record already read. */
function matchDamage(steamId: string, details: ReadonlyMap<string, LibraryMatch>): ReadonlyMap<string, MatchDamage> {
  const out = new Map<string, MatchDamage>();
  for (const [id, match] of details) {
    const own = match.players?.find((player) => player.steamid64 === steamId);
    if (own) out.set(id, { damage: own.damage, rounds: own.rounds });
  }
  return out;
}

/** Where the reference comes from; says so when the pros have no entry for the scope's map. */
function sourceText(baseline: Baseline, map: string): string {
  const otherMap = map !== '' && baseline.map !== map ? ` No hay referencia pro de ${prettyMapName(map)}: se compara con todos sus mapas.` : '';
  if (baseline.scope === 'pro') return `${proSourceText(baseline)}${otherMap}`;
  return `La media sale de ${baselineSampleText(baseline)}${map === '' ? '' : ` en ${prettyMapName(map)}`}.`;
}

/**
 * The player at a glance: the reference metrics against the library or the
 * pros, and the recent matches as a chart with its average.
 */
export function PlayerIntro({
  report,
  details,
  map,
  bothSides,
  reference,
}: {
  report: MetricsReport;
  details: ReadonlyMap<string, LibraryMatch>;
  /** The scope's map; the reference reads the same map. */
  map: string;
  bothSides: boolean;
  /** The context bar's choice. */
  reference: BaselineScope;
}): ReactNode {
  const references = useReferences(map);
  const [trend, setTrend] = useState<TrendMetric>('kd');
  const player = report.players[0];
  if (!player) return null;
  const ready = references !== null && references !== 'failed' ? references : null;
  const scope = reference;
  const baseline = ready ? ready[scope] : null;
  const values = playerReferenceValues(player);
  const evidence = playerEvidence(player);
  const damage = matchDamage(player.steamid64, details);
  const label = `${referenceLabel(scope)}${bothSides ? '' : ', ambos lados'}`;
  const sparkline = (metric: TrendMetric): number[] | undefined => {
    const points = trendPoints(player.match_lines, metric, damage);
    return points.length >= MIN_TREND_POINTS ? points.map((point) => point.value) : undefined;
  };
  const trends: Partial<Record<BaselineMetric, number[] | undefined>> = {
    kpr: sparkline('kpr'),
    adr: sparkline('adr'),
    wpa_per_round: sparkline('wpa'),
  };
  const reliabilities = BASELINE_METRICS.map((metric) => metricReliability(evidence[metric], baseline?.metrics[metric], metric));
  // When every figure is thin, the note under the cards says it once instead of each card.
  const allWeak = reliabilities.length > 0 && reliabilities.every((r) => r?.weak === true);
  const chartPoints = trendPoints(player.match_lines, trend, damage);
  const chartBaseline = TREND_BASELINE[trend];
  const chartReference = chartBaseline === null ? null : baselineValue(baseline, chartBaseline);

  return (
    <div className="flex flex-col gap-3">
      <section aria-label="Cifras frente a la referencia" className="@container flex flex-col gap-1.5">
        <SlashTitle>Frente a la {referenceLabel(scope).toLowerCase()}</SlashTitle>
        {references === 'failed' ? (
          <p role="alert" className="text-body-sm text-destructive">
            No se pudo leer la media de la biblioteca. Las cifras salen sin comparar.
          </p>
        ) : null}
        <EdgeTileGrid label="Cifras principales" className="grid-cols-[repeat(auto-fill,minmax(9.5rem,1fr))]">
          {BASELINE_METRICS.map((metric, index) => {
            const info = METRIC_INFO[metric];
            const cardReliability = reliabilities[index] ?? null;
            return (
              <PlayerMetricTile
                key={metric}
                label={info.label}
                value={values[metric] ?? null}
                format={info.format}
                higherIsBetter={info.higherIsBetter}
                epsilon={info.epsilon}
                mean={baselineValue(baseline, metric)}
                meanLabel={label}
                reliability={cardReliability && allWeak ? { ...cardReliability, weak: false } : cardReliability}
                trend={trends[metric]}
              />
            );
          })}
        </EdgeTileGrid>
        <p className="measure-read text-meta text-fg-3">
          {player.rounds} rondas en {report.matches} {report.matches === 1 ? 'partida' : 'partidas'}.{allWeak ? ' Poca muestra en todas las cifras: pesa más la media que sus rondas.' : ''} ▲▼ solo cuando la diferencia es fiable.
          {baseline ? ` ${sourceText(baseline, map)}` : ''}
          {ready && scope === 'library' ? minRoundsNote(ready.minRounds) : ''}
          {references === null ? ' Leyendo la media.' : ''}
        </p>
      </section>
      <section aria-label="Rendimiento reciente" className="studio-panel flex flex-col gap-2 px-3 py-2.5">
        <SlashTitle>Rendimiento reciente</SlashTitle>
        {chartPoints.length < MIN_TREND_POINTS ? (
          <p className="text-body-sm text-fg-2">Hacen falta al menos dos partidas con este filtro para dibujar la evolución.</p>
        ) : (
          <LineChart
            label={`${TREND_LABEL[trend]} por partida, la más antigua primero`}
            categories={chartPoints.map((point) => ({
              label: `${prettyMapName(point.map)} ${formatShortDate(point.playedAt)}`,
              detail: `${dateText(point.playedAt)} · ${point.rounds} rondas`,
            }))}
            series={[{ id: trend, label: TREND_LABEL[trend], values: chartPoints.map((point) => point.value) }]}
            format={TREND_FORMAT[trend]}
            showAverage
            reference={chartReference === null ? undefined : { value: chartReference, label: SCOPE_LABEL[scope] }}
            toolbar={
              <Segmented
                label="Cifra del gráfico"
                value={trend}
                options={TREND_METRICS.map((value) => ({ value, label: TREND_LABEL[value] }))}
                onChange={setTrend}
              />
            }
          />
        )}
      </section>
    </div>
  );
}
