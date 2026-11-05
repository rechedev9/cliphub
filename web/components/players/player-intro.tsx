'use client';

import { useEffect, useState, type ReactNode } from 'react';
import { getBaseline, type LibraryMatch } from '@/lib/api/library';
import { BASELINE_METRICS, type Baseline, type BaselineMetric, type BaselineScope } from '@/lib/api/baseline';
import type { MetricsReport } from '@/lib/api/metrics';
import {
  METRIC_INFO,
  SCOPE_LABEL,
  baselineSampleText,
  baselineValue,
  playerReferenceValues,
  proUnavailableText,
  referenceLabel,
} from '@/lib/analysis/reference';
import { TREND_BASELINE, TREND_LABEL, TREND_METRICS, trendPoints, type MatchDamage, type TrendMetric } from '@/lib/analysis/player-trend';
import { prettyMapName } from '@/lib/format';
import { dateText } from '@/lib/reports/scout-sections';
import { LineChart } from '@/components/ui/line-chart';
import { Segmented } from '@/components/ui/segmented';
import { StatCard, StatCardGroup } from '@/components/ui/stat-card';

type References = { library: Baseline; pro: Baseline } | null | 'failed';

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

function useReferences(map: string): References {
  const [references, setReferences] = useState<References>(null);
  useEffect(() => {
    let active = true;
    setReferences(null);
    Promise.all([getBaseline({ scope: 'library', map }), getBaseline({ scope: 'pro', map })])
      .then(([library, pro]) => {
        if (active) setReferences({ library, pro });
      })
      .catch(() => {
        if (active) setReferences('failed');
      });
    return () => {
      active = false;
    };
  }, [map]);
  return references;
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
}: {
  report: MetricsReport;
  details: ReadonlyMap<string, LibraryMatch>;
  /** The scope's map; the reference reads the same map. */
  map: string;
  bothSides: boolean;
}): ReactNode {
  const references = useReferences(map);
  const [chosen, setChosen] = useState<BaselineScope>('library');
  const [trend, setTrend] = useState<TrendMetric>('kd');
  const player = report.players[0];
  if (!player) return null;
  const ready = references !== null && references !== 'failed' ? references : null;
  const proMissing = ready ? proUnavailableText(ready.pro) : null;
  const scope: BaselineScope = proMissing === null ? chosen : 'library';
  const baseline = ready ? ready[scope] : null;
  const values = playerReferenceValues(player);
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
  const chartPoints = trendPoints(player.match_lines, trend, damage);
  const chartBaseline = TREND_BASELINE[trend];
  const chartReference = chartBaseline === null ? null : baselineValue(baseline, chartBaseline);

  return (
    <div className="flex flex-col gap-5">
      <section aria-label="Cifras frente a la referencia" className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-body font-semibold text-fg-1">Frente a la referencia</h2>
          {proMissing === null ? (
            <Segmented
              label="Referencia"
              value={scope}
              options={(['library', 'pro'] as const).map((value) => ({ value, label: SCOPE_LABEL[value] }))}
              onChange={setChosen}
            />
          ) : null}
        </div>
        {references === 'failed' ? (
          <p role="alert" className="text-body-sm text-destructive">
            No se pudo leer la media de la biblioteca. Las cifras salen sin comparar.
          </p>
        ) : null}
        <StatCardGroup label="Cifras principales">
          {BASELINE_METRICS.map((metric) => {
            const info = METRIC_INFO[metric];
            const reference = baselineValue(baseline, metric);
            return (
              <StatCard
                key={metric}
                label={info.label}
                value={values[metric] ?? null}
                format={info.format}
                higherIsBetter={info.higherIsBetter}
                epsilon={info.epsilon}
                comparison={reference === null ? undefined : { value: reference, label }}
                trend={trends[metric]}
              />
            );
          })}
        </StatCardGroup>
        <p className="measure-read text-meta text-fg-3">
          {player.rounds} rondas en {report.matches} {report.matches === 1 ? 'partida' : 'partidas'}.
          {baseline ? ` La media sale de ${baselineSampleText(baseline)}${map === '' ? '' : ` en ${prettyMapName(map)}`}.` : ''}
          {references === null ? ' Leyendo la media.' : ''}
          {proMissing ? ` ${proMissing}` : ''}
        </p>
      </section>
      <section aria-label="Rendimiento reciente" className="studio-panel flex flex-col gap-3 px-4 py-3">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="text-body-sm font-semibold text-fg-1">Rendimiento reciente</h2>
          <Segmented
            label="Cifra del gráfico"
            value={trend}
            options={TREND_METRICS.map((value) => ({ value, label: TREND_LABEL[value] }))}
            onChange={setTrend}
          />
        </div>
        {chartPoints.length < MIN_TREND_POINTS ? (
          <p className="text-body-sm text-fg-2">Hacen falta al menos dos partidas con este filtro para dibujar la evolución.</p>
        ) : (
          <LineChart
            label={`${TREND_LABEL[trend]} por partida, la más antigua primero`}
            points={chartPoints.map((point) => ({
              label: `${prettyMapName(point.map)}, ${dateText(point.playedAt)}`,
              value: point.value,
              detail: `${point.rounds} rondas`,
            }))}
            format={TREND_FORMAT[trend]}
            showAverage
            reference={chartReference === null ? undefined : { value: chartReference, label: referenceLabel(scope) }}
          />
        )}
      </section>
    </div>
  );
}
