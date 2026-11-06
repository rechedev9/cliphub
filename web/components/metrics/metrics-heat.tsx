'use client';

import type { ReactNode } from 'react';
import type { MetricsReport } from '@/lib/api/metrics';
import { buyIcon } from '@/lib/analysis/icons';
import { rateEstimate, reliability, roundPriors, roundReferences } from '@/lib/analysis/reference';
import { ratioEstimate } from '@/lib/analysis/shrinkage';
import { prettyMapName } from '@/lib/format';
import { ecoText, mapLines, percentText, ppText, type MetricsSectionOptions } from '@/lib/metrics/advanced';
import { buyLabel } from '@/lib/tactical-labels';
import { HeatTable, type HeatColumn, type HeatRow } from '@/components/ui/heat-table';
import { useLibraryBaselines } from '@/hooks/use-references';

const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** Below this many rounds a per-buy or per-map number is dimmed. */
const MIN_BUY_ROUNDS = 10;
const MIN_MAP_ROUNDS = 20;

const ratio = (value: number, rounds: number): number | null => (rounds > 0 ? value / rounds : null);

/** Every round has a winner, so the library wins half of its rounds. */
const LIBRARY_WIN_SHARE = 0.5;

/** The tint of a column against a library mean; a column the library cannot read stays plain. */
function libraryHeat(baseline: number | null, spread: number, higherIsBetter = true): HeatColumn['heat'] {
  return baseline === null ? undefined : { baseline, spread, higherIsBetter };
}

function referenceText(minRounds: number | null): string {
  if (minRounds === null) return 'la media de la biblioteca, que aún se está leyendo';
  return `la media de los jugadores de la biblioteca con ${minRounds} rondas o más (en % ganadas, el 50 %)`;
}

/** The report's map when it reads one, else every map. */
function reportMap(report: MetricsReport): string {
  const maps = new Set(mapLines(report).map((line) => line.map));
  return maps.size === 1 ? ([...maps][0] ?? '') : '';
}

/** Every player's rounds per buy type, tinted against the library's player means. */
export function BuyHeatTable({ report, options }: { report: MetricsReport; options: MetricsSectionOptions }): ReactNode {
  const map = reportMap(report);
  const baselines = useLibraryBaselines([map]);
  const baseline = baselines?.get(map) ?? null;
  const refs = roundReferences(baseline);
  const priors = roundPriors(baseline);
  const lines = report.players.flatMap((player) => player.buys.map((line) => ({ player, line })));
  const many = report.players.length > 1;
  const columns: HeatColumn[] = [
    ...(many ? [{ id: 'player', header: 'Jugador' }] : []),
    { id: 'buy', header: 'Compra' },
    { id: 'won', header: '% ganadas', align: 'end', heat: { baseline: LIBRARY_WIN_SHARE, spread: 0.15 } },
    { id: 'kills', header: 'Bajas por ronda', align: 'end', heat: libraryHeat(refs.kills, 0.25) },
    { id: 'deaths', header: 'Muertes por ronda', align: 'end', heat: libraryHeat(refs.deaths, 0.25, false) },
    { id: 'wpa', header: 'WPA por ronda', align: 'end', heat: libraryHeat(refs.wpa, 0.03) },
  ];
  const rows: HeatRow[] = lines.map(({ player, line }) => {
    const n = line.rounds;
    const won = ratio(line.won, n);
    const kills = ratio(line.kills, n);
    const deaths = ratio(line.deaths, n);
    const wpa = ratio(line.wpa, n);
    const name = player.name || player.steamid64;
    const format2 = (v: number): string => decimal2.format(v);
    return {
      id: `${player.steamid64}:${line.buy}`,
      cells: {
        player: { text: name, href: options.playerHref?.(player.steamid64) },
        buy: { text: buyLabel(line.buy), icon: buyIcon(line.buy) },
        won: { text: won === null ? '' : percentText(won), value: won, n },
        kills: { text: kills === null ? '' : decimal2.format(kills), value: kills, n, reliability: reliability(rateEstimate(kills, n, priors.kills), refs.kills, format2) },
        deaths: { text: deaths === null ? '' : decimal2.format(deaths), value: deaths, n, reliability: reliability(rateEstimate(deaths, n, priors.deaths), refs.deaths, format2) },
        wpa: { text: wpa === null ? '' : ppText(wpa), value: wpa, n, reliability: reliability(rateEstimate(wpa, n, priors.wpa), refs.wpa, ppText) },
      },
    };
  });
  return (
    <HeatTable
      label="Rendimiento por tipo de compra"
      columns={columns}
      rows={rows}
      minSample={MIN_BUY_ROUNDS}
      sampleNoun="rondas"
      empty="Sin rondas en la selección."
      reference={referenceText(baseline?.min_rounds ?? null)}
      reliableOnly
    />
  );
}

/**
 * One player's matches added up per map, each row tinted against the library's
 * player means on that map. The adjusted kills have no library mean and stay plain.
 */
export function MapHeatTable({ report }: { report: MetricsReport }): ReactNode {
  const lines = mapLines(report);
  const baselines = useLibraryBaselines(lines.map((line) => line.map));
  const columns: HeatColumn[] = [
    { id: 'map', header: 'Mapa' },
    { id: 'matches', header: 'Partidas', align: 'end' },
    { id: 'kd', header: 'K/D', align: 'end', heat: { baseline: 1, spread: 0.3 } },
    { id: 'eco', header: 'Bajas ajustadas por ronda', align: 'end' },
    { id: 'wpa', header: 'WPA por ronda', align: 'end', heat: { baseline: 0, spread: 0.03 } },
  ];
  const rows: HeatRow[] = lines.map((line) => {
    const n = line.rounds;
    const lineKd = line.deaths > 0 ? line.kills / line.deaths : line.kills;
    const eco = ratio(line.ecoKills, n);
    const wpa = ratio(line.wpa, n);
    const refs = roundReferences(baselines?.get(line.map));
    const priors = roundPriors(baselines?.get(line.map));
    const killRate = rateEstimate(ratio(line.kills, n), n, priors.kills);
    const deathRate = rateEstimate(ratio(line.deaths, n), n, priors.deaths);
    const kd = killRate && deathRate ? ratioEstimate(killRate, deathRate) : null;
    const place = prettyMapName(line.map);
    return {
      id: line.map,
      cells: {
        map: { text: place, map: line.map },
        matches: { text: String(line.matches) },
        kd: {
          text: decimal2.format(lineKd),
          value: lineKd,
          n,
          reference: refs.kd,
          referenceText: refs.kd === null ? undefined : `Biblioteca en ${place}: ${decimal2.format(refs.kd)}`,
          reliability: reliability(kd, refs.kd, (v) => decimal2.format(v)),
        },
        eco: { text: eco === null ? '' : ecoText(eco), n },
        wpa: {
          text: wpa === null ? '' : ppText(wpa),
          value: wpa,
          n,
          reference: refs.wpa,
          referenceText: refs.wpa === null ? undefined : `Biblioteca en ${place}: ${ppText(refs.wpa)}`,
          reliability: reliability(rateEstimate(wpa, n, priors.wpa), refs.wpa, ppText),
        },
      },
    };
  });
  const minRounds = baselines === null ? null : ([...baselines.values()][0]?.min_rounds ?? null);
  return (
    <HeatTable
      label="Rendimiento por mapa"
      columns={columns}
      rows={rows}
      minSample={MIN_MAP_ROUNDS}
      sampleNoun="rondas"
      empty="Sin partidas leídas."
      reference={minRounds === null ? 'la media de la biblioteca, que aún se está leyendo' : `la media de la biblioteca en cada mapa, sus jugadores con ${minRounds} rondas o más`}
      reliableOnly
    />
  );
}
