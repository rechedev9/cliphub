import type { ReactNode } from 'react';
import type { MetricsReport } from '@/lib/api/metrics';
import { weightedMean } from '@/lib/analysis/compare';
import { prettyMapName } from '@/lib/format';
import { ecoText, mapLines, percentText, ppText, type MetricsSectionOptions } from '@/lib/metrics/advanced';
import { buyLabel } from '@/lib/tactical-labels';
import { HeatTable, type HeatColumn, type HeatRow } from '@/components/ui/heat-table';

const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** Below this many rounds a per-buy or per-map number is dimmed. */
const MIN_BUY_ROUNDS = 10;
const MIN_MAP_ROUNDS = 20;

const ratio = (value: number, rounds: number): number | null => (rounds > 0 ? value / rounds : null);

/** Every player's rounds per buy type, tinted against the table's own weighted mean. */
export function BuyHeatTable({ report, options }: { report: MetricsReport; options: MetricsSectionOptions }): ReactNode {
  const lines = report.players.flatMap((player) => player.buys.map((line) => ({ player, line })));
  const many = report.players.length > 1;
  const mean = (value: (l: (typeof lines)[number]) => number): number =>
    weightedMean(lines, (l) => value(l) / l.line.rounds, (l) => l.line.rounds) ?? 0;
  const columns: HeatColumn[] = [
    ...(many ? [{ id: 'player', header: 'Jugador' }] : []),
    { id: 'buy', header: 'Compra' },
    { id: 'won', header: '% ganadas', align: 'end', heat: { baseline: mean((l) => l.line.won), spread: 0.15 } },
    { id: 'kills', header: 'Bajas por ronda', align: 'end', heat: { baseline: mean((l) => l.line.kills), spread: 0.25 } },
    {
      id: 'deaths',
      header: 'Muertes por ronda',
      align: 'end',
      heat: { baseline: mean((l) => l.line.deaths), spread: 0.25, higherIsBetter: false },
    },
    { id: 'wpa', header: 'WPA por ronda', align: 'end', heat: { baseline: mean((l) => l.line.wpa), spread: 0.03 } },
  ];
  const rows: HeatRow[] = lines.map(({ player, line }) => {
    const n = line.rounds;
    const won = ratio(line.won, n);
    const kills = ratio(line.kills, n);
    const deaths = ratio(line.deaths, n);
    const wpa = ratio(line.wpa, n);
    const name = player.name || player.steamid64;
    return {
      id: `${player.steamid64}:${line.buy}`,
      cells: {
        player: { text: name, href: options.playerHref?.(player.steamid64) },
        buy: { text: buyLabel(line.buy) },
        won: { text: won === null ? '' : percentText(won), value: won, n },
        kills: { text: kills === null ? '' : decimal2.format(kills), value: kills, n },
        deaths: { text: deaths === null ? '' : decimal2.format(deaths), value: deaths, n },
        wpa: { text: wpa === null ? '' : ppText(wpa), value: wpa, n },
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
    />
  );
}

/** One player's matches added up per map, tinted against their own weighted mean. */
export function MapHeatTable({ report }: { report: MetricsReport }): ReactNode {
  const lines = mapLines(report);
  const mean = (value: (l: (typeof lines)[number]) => number): number => weightedMean(lines, (l) => value(l) / l.rounds, (l) => l.rounds) ?? 0;
  const deaths = lines.reduce((sum, l) => sum + l.deaths, 0);
  const kd = deaths > 0 ? lines.reduce((sum, l) => sum + l.kills, 0) / deaths : 1;
  const columns: HeatColumn[] = [
    { id: 'map', header: 'Mapa' },
    { id: 'matches', header: 'Partidas', align: 'end' },
    { id: 'kd', header: 'K/D', align: 'end', heat: { baseline: kd, spread: 0.3 } },
    { id: 'eco', header: 'Bajas ajustadas por ronda', align: 'end', heat: { baseline: mean((l) => l.ecoKills), spread: 0.2 } },
    { id: 'wpa', header: 'WPA por ronda', align: 'end', heat: { baseline: mean((l) => l.wpa), spread: 0.03 } },
  ];
  const rows: HeatRow[] = lines.map((line) => {
    const n = line.rounds;
    const lineKd = line.deaths > 0 ? line.kills / line.deaths : line.kills;
    const eco = ratio(line.ecoKills, n);
    const wpa = ratio(line.wpa, n);
    return {
      id: line.map,
      cells: {
        map: { text: prettyMapName(line.map) },
        matches: { text: String(line.matches) },
        kd: { text: decimal2.format(lineKd), value: lineKd, n },
        eco: { text: eco === null ? '' : ecoText(eco), value: eco, n },
        wpa: { text: wpa === null ? '' : ppText(wpa), value: wpa, n },
      },
    };
  });
  return (
    <HeatTable
      label="Rendimiento por mapa"
      columns={columns}
      rows={rows}
      minSample={MIN_MAP_ROUNDS}
      sampleNoun="rondas"
      empty="Sin partidas leídas."
    />
  );
}
