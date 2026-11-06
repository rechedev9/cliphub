import type { Baseline, BaselineMetric, BaselineScope, Leaderboard } from '../api/baseline.ts';
import type { PlayerMetrics } from '../api/metrics.ts';

/**
 * The reference metrics as people read them: label, format and direction,
 * and a player's own value computed the way `tacticalplan.playerTotals` does,
 * so a card compares like with like.
 */

const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const decimal1 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 1 });

function pp(value: number): string {
  const points = Math.round(value * 1000) / 10;
  let sign = '';
  if (points > 0) sign = '+';
  else if (points < 0) sign = '-';
  return `${sign}${decimal1.format(Math.abs(points))} pp`;
}

export type MetricInfo = {
  readonly label: string;
  readonly format: (value: number) => string;
  readonly higherIsBetter: boolean;
  /** Differences under this read as even. */
  readonly epsilon: number;
};

export const METRIC_INFO: Readonly<Record<BaselineMetric, MetricInfo>> = {
  kpr: { label: 'Bajas por ronda', format: (v) => decimal2.format(v), higherIsBetter: true, epsilon: 0.005 },
  dpr: { label: 'Muertes por ronda', format: (v) => decimal2.format(v), higherIsBetter: false, epsilon: 0.005 },
  adr: { label: 'ADR', format: (v) => decimal1.format(v), higherIsBetter: true, epsilon: 0.05 },
  kast: { label: 'KAST', format: (v) => `${decimal1.format(v)} %`, higherIsBetter: true, epsilon: 0.05 },
  opening_win_pct: { label: 'Duelos de apertura ganados', format: (v) => `${decimal1.format(v)} %`, higherIsBetter: true, epsilon: 0.05 },
  trade_pct: { label: 'Muertes tradeadas', format: (v) => `${decimal1.format(v)} %`, higherIsBetter: true, epsilon: 0.05 },
  utility_damage_per_round: { label: 'Daño de granadas por ronda', format: (v) => decimal1.format(v), higherIsBetter: true, epsilon: 0.05 },
  flash_assists_per_round: { label: 'Asistencias de flash por ronda', format: (v) => decimal2.format(v), higherIsBetter: true, epsilon: 0.005 },
  wpa_per_round: { label: 'WPA por ronda', format: pp, higherIsBetter: true, epsilon: 0.0005 },
};

export const SCOPE_LABEL: Readonly<Record<BaselineScope, string>> = { library: 'Biblioteca', pro: 'Pro' };

/** The comparison line under a stat card, such as "media de la biblioteca". */
export function referenceLabel(scope: BaselineScope): string {
  return scope === 'pro' ? 'Media de los pros' : 'Media de la biblioteca';
}

/** A player's value of each reference metric; a metric without a denominator is left out. */
export function playerReferenceValues(p: PlayerMetrics): Partial<Record<BaselineMetric, number>> {
  const out: Partial<Record<BaselineMetric, number>> = {};
  if (p.rounds <= 0) return out;
  out.kpr = p.kills / p.rounds;
  out.dpr = p.deaths / p.rounds;
  out.adr = p.box.damage / p.rounds;
  out.kast = (100 * p.box.kast_rounds) / p.rounds;
  out.wpa_per_round = p.swing.wpa / p.rounds;
  const duels = p.box.opening_kills + p.box.opening_deaths;
  if (duels > 0) out.opening_win_pct = (100 * p.box.opening_kills) / duels;
  if (p.deaths > 0) out.trade_pct = (100 * p.trades.traded_deaths.count) / p.deaths;
  if (p.box.utility_rounds > 0) {
    out.utility_damage_per_round = (p.utility.he.enemy_damage + p.utility.molotov.enemy_damage) / p.box.utility_rounds;
    out.flash_assists_per_round = p.utility.flash.assists.count / p.box.utility_rounds;
  }
  return out;
}

/** The reference value of a metric, or null when no qualifying player has it. */
export function baselineValue(baseline: Baseline | null, metric: BaselineMetric): number | null {
  const entry = baseline?.metrics[metric];
  return entry && entry.players > 0 ? entry.mean : null;
}

/** Why Pro cannot be chosen, in plain words; null when it can. */
export function proUnavailableText(pro: Baseline | null): string | null {
  if (pro === null || pro.players > 0) return null;
  if (pro.matches === 0) return 'Pro no está disponible: la biblioteca no tiene demos de HLTV con este filtro.';
  return `Pro no está disponible: ningún jugador de las demos de HLTV llega a ${pro.min_rounds} rondas con este filtro.`;
}

/** Why a ranking has no rows, in plain words. */
export function leaderboardEmptyText(board: Pick<Leaderboard, 'scope' | 'matches' | 'min_rounds'>): string {
  if (board.matches === 0) {
    return board.scope === 'pro'
      ? 'La biblioteca no tiene demos de HLTV con este mapa. Importa demos pro o cambia a Biblioteca.'
      : 'No hay partidas analizadas con este mapa. Importa demos o elige otro mapa.';
  }
  return `Ningún jugador llega a ${board.min_rounds} rondas con este filtro. Baja el mínimo de rondas.`;
}

/** "238 jugadores con 50 rondas o más", the sample behind a reference. */
export function baselineSampleText(baseline: Baseline): string {
  const who = baseline.players === 1 ? '1 jugador' : `${baseline.players} jugadores`;
  return `${who} con ${baseline.min_rounds} rondas o más en ${baseline.matches} ${baseline.matches === 1 ? 'partida' : 'partidas'}`;
}

/** Says why the minimum is under 50 rounds; empty when it is not. */
export function minRoundsNote(minRounds: number): string {
  return minRounds < 50 ? ` El mínimo baja de 50 a ${minRounds} rondas porque la biblioteca aún es pequeña.` : '';
}
