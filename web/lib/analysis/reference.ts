import type { Baseline, BaselineMetric, BaselineScope, BaselineValue } from '../api/baseline.ts';
import type { PlayerMetrics } from '../api/metrics.ts';
import { differsFrom, estimate, estimateText, WEAK_WEIGHT, type Estimate, type Evidence, type Prior } from './shrinkage.ts';

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

/** The counts behind each reference value, for shrinkage; same denominators as `playerReferenceValues`. */
export function playerEvidence(p: PlayerMetrics): Partial<Record<BaselineMetric, Evidence>> {
  const out: Partial<Record<BaselineMetric, Evidence>> = {};
  if (p.rounds <= 0) return out;
  const rate = (value: number, rounds: number): Evidence => ({ kind: 'rate', value: value / rounds, rounds });
  out.kpr = rate(p.kills, p.rounds);
  out.dpr = rate(p.deaths, p.rounds);
  out.adr = rate(p.box.damage, p.rounds);
  out.wpa_per_round = rate(p.swing.wpa, p.rounds);
  out.kast = { kind: 'share', successes: p.box.kast_rounds, trials: p.rounds };
  const duels = p.box.opening_kills + p.box.opening_deaths;
  if (duels > 0) out.opening_win_pct = { kind: 'share', successes: p.box.opening_kills, trials: duels };
  if (p.deaths > 0) out.trade_pct = { kind: 'share', successes: p.trades.traded_deaths.count, trials: p.deaths };
  if (p.box.utility_rounds > 0) {
    out.utility_damage_per_round = rate(p.utility.he.enemy_damage + p.utility.molotov.enemy_damage, p.box.utility_rounds);
    out.flash_assists_per_round = rate(p.utility.flash.assists.count, p.box.utility_rounds);
  }
  return out;
}

/** The prior a baseline value gives shrinkage; null when no player has the metric. */
export function baselinePrior(value: BaselineValue | undefined): Prior | null {
  if (!value || value.players <= 0) return null;
  return { mean: value.mean, priorStddev: value.prior_stddev, roundStddev: value.round_stddev };
}

/** What a stat card or heat cell needs to call a comparison: whether it holds, the tooltip, and the weak-sample marker. */
export type Reliability = { readonly differs: boolean; readonly detail: string; readonly weak: boolean };

/** The reliability of a value against a reference mean; null when it cannot be estimated. */
export function reliability(e: Estimate | null, reference: number | null, format: (value: number) => string): Reliability | null {
  if (e === null || reference === null) return null;
  return { differs: differsFrom(e, reference), detail: estimateText(e, format), weak: e.weight < WEAK_WEIGHT };
}

/** The reliability of a player's metric against a baseline. */
export function metricReliability(evidence: Evidence | undefined, value: BaselineValue | undefined, metric: BaselineMetric): Reliability | null {
  const prior = baselinePrior(value);
  if (!evidence || prior === null) return null;
  return reliability(estimate(evidence, prior), prior.mean, METRIC_INFO[metric].format);
}

/** Where the Pro numbers come from, said once beside the comparison. */
export function proSourceText(baseline: Baseline): string {
  if (baseline.source === 'server') return `${proSampleText(baseline)}. ${baseline.description ?? ''}`.trim();
  if (baseline.source === 'bundled') return `Referencia pro incluida en Studio: ${baseline.description ?? 'partidas profesionales'}`;
  return `Tus demos de HLTV: ${baselineSampleText(baseline)}.`;
}

const updatedDate = new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });

/** "Referencia pro: 283 mapas, 6120 rondas, actualizada 6 nov 2026", the sample behind the server reference. */
export function proSampleText(baseline: Pick<Baseline, 'matches' | 'rounds' | 'updated_at'>): string {
  const maps = `${baseline.matches} ${baseline.matches === 1 ? 'mapa' : 'mapas'}`;
  const rounds = `${baseline.rounds.toLocaleString('es-ES')} rondas`;
  const date = baseline.updated_at ? Date.parse(baseline.updated_at) : Number.NaN;
  return `Referencia pro: ${maps}, ${rounds}${Number.isNaN(date) ? '' : `, actualizada ${updatedDate.format(date)}`}`;
}

/** "4 mapas pro incluidos" or "12 demos de HLTV": the source in the few words the context bar has room for. */
export function proReferenceShort(baseline: Baseline): string {
  const n = baseline.matches;
  if (baseline.source === 'server') return `${n} ${n === 1 ? 'mapa pro' : 'mapas pro'}`;
  if (baseline.source === 'bundled') return `${n} ${n === 1 ? 'mapa pro incluido' : 'mapas pro incluidos'}`;
  return `${n} ${n === 1 ? 'demo de HLTV' : 'demos de HLTV'}`;
}

/** The reference value of a metric, or null when no qualifying player has it. */
export function baselineValue(baseline: Baseline | null, metric: BaselineMetric): number | null {
  const entry = baseline?.metrics[metric];
  return entry && entry.players > 0 ? entry.mean : null;
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

/** The priors behind `roundReferences`, for the reliability of a per-round heat cell. */
export type RoundPriors = { readonly kills: Prior | null; readonly deaths: Prior | null; readonly wpa: Prior | null };

export function roundPriors(baseline: Baseline | null | undefined): RoundPriors {
  return {
    kills: baselinePrior(baseline?.metrics.kpr),
    deaths: baselinePrior(baseline?.metrics.dpr),
    wpa: baselinePrior(baseline?.metrics.wpa_per_round),
  };
}

/** The estimate of a per-round rate over `rounds`; null without a value or a prior. */
export function rateEstimate(value: number | null, rounds: number, prior: Prior | null): Estimate | null {
  return value === null || prior === null ? null : estimate({ kind: 'rate', value, rounds }, prior);
}

/** Per-round references a heat table tints against: the library baseline's means, null where it has no players. */
export type RoundReferences = { readonly kills: number | null; readonly deaths: number | null; readonly wpa: number | null; readonly kd: number | null };

export function roundReferences(baseline: Baseline | null | undefined): RoundReferences {
  const kills = baselineValue(baseline ?? null, 'kpr');
  const deaths = baselineValue(baseline ?? null, 'dpr');
  return { kills, deaths, wpa: baselineValue(baseline ?? null, 'wpa_per_round'), kd: kills !== null && deaths !== null && deaths > 0 ? kills / deaths : null };
}
