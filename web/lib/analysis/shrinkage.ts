/**
 * Empirical Bayes for small samples: a player's value is pulled toward the
 * reference by as much as their rounds leave in doubt, and a 90 % interval
 * says how far the true value may sit. A comparison is called only when that
 * interval leaves the reference out.
 */

/** A percentage as successes out of trials, or a per-round rate over its rounds. */
export type Evidence =
  | { readonly kind: 'share'; readonly successes: number; readonly trials: number }
  | { readonly kind: 'rate'; readonly value: number; readonly rounds: number };

/** The reference population, in the metric's own units (0 to 100 for a percentage). */
export type Prior = { readonly mean: number; readonly priorStddev: number; readonly roundStddev?: number };

export type Estimate = {
  /** The value as measured. */
  readonly raw: number;
  /** The value pulled toward the reference. */
  readonly shrunk: number;
  /** The 90 % interval of the true value. */
  readonly low: number;
  readonly high: number;
  /** The posterior standard deviation, for ratios of two estimates. */
  readonly sd: number;
  /** The share the player's own rounds carry, from 0 to 1; the rest is the reference. */
  readonly weight: number;
};

const Z90 = 1.6448536269514722;
/** Under this weight the reference outweighs the player's own rounds. */
export const WEAK_WEIGHT = 0.5;

function logGamma(x: number): number {
  const g = 7;
  const c = [
    0.9999999999998099, 676.5203681218851, -1259.1392167224028, 771.3234287776531, -176.6150291621406, 12.507343278686905,
    -0.13857109526572012, 9.984369578019572e-6, 1.5056327351493116e-7,
  ];
  if (x < 0.5) return Math.log(Math.PI / Math.sin(Math.PI * x)) - logGamma(1 - x);
  const z = x - 1;
  let a = c[0] ?? 0;
  const t = z + g + 0.5;
  for (let i = 1; i < g + 2; i++) a += (c[i] ?? 0) / (z + i);
  return 0.5 * Math.log(2 * Math.PI) + (z + 0.5) * Math.log(t) - t + Math.log(a);
}

/** The continued fraction of the incomplete beta function (modified Lentz). */
function betaFraction(x: number, a: number, b: number): number {
  const tiny = 1e-300;
  let c = 1;
  let d = 1 - ((a + b) * x) / (a + 1);
  if (Math.abs(d) < tiny) d = tiny;
  d = 1 / d;
  let h = d;
  for (let m = 1; m <= 300; m++) {
    const m2 = 2 * m;
    let aa = (m * (b - m) * x) / ((a + m2 - 1) * (a + m2));
    d = 1 + aa * d;
    if (Math.abs(d) < tiny) d = tiny;
    c = 1 + aa / c;
    if (Math.abs(c) < tiny) c = tiny;
    d = 1 / d;
    h *= d * c;
    aa = (-(a + m) * (a + b + m) * x) / ((a + m2) * (a + m2 + 1));
    d = 1 + aa * d;
    if (Math.abs(d) < tiny) d = tiny;
    c = 1 + aa / c;
    if (Math.abs(c) < tiny) c = tiny;
    d = 1 / d;
    const step = d * c;
    h *= step;
    if (Math.abs(step - 1) < 1e-12) break;
  }
  return h;
}

/** The regularized incomplete beta function I_x(a, b): the Beta(a, b) CDF at x. */
export function betaCdf(x: number, a: number, b: number): number {
  if (x <= 0) return 0;
  if (x >= 1) return 1;
  const front = Math.exp(logGamma(a + b) - logGamma(a) - logGamma(b) + a * Math.log(x) + b * Math.log(1 - x));
  if (x < (a + 1) / (a + b + 2)) return (front * betaFraction(x, a, b)) / a;
  return 1 - (front * betaFraction(1 - x, b, a)) / b;
}

/** The p quantile of Beta(a, b), by bisection. */
export function betaQuantile(p: number, a: number, b: number): number {
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 60; i++) {
    const mid = (lo + hi) / 2;
    if (betaCdf(mid, a, b) < p) lo = mid;
    else hi = mid;
  }
  return (lo + hi) / 2;
}

/** Beta-binomial: the reference as a Beta prior with its mean and spread, updated by the player's counts. */
function shareEstimate(successes: number, trials: number, prior: Prior): Estimate | null {
  if (!(trials > 0) || !(prior.priorStddev > 0)) return null;
  const m = Math.min(Math.max(prior.mean / 100, 0.001), 0.999);
  const s = prior.priorStddev / 100;
  // A spread wider than a Beta can hold leaves the prior at the weight of one trial.
  const strength = Math.max(1, (m * (1 - m)) / (s * s) - 1);
  const a = m * strength + successes;
  const b = (1 - m) * strength + (trials - successes);
  const total = a + b;
  return {
    raw: (100 * successes) / trials,
    shrunk: (100 * a) / total,
    low: 100 * betaQuantile(0.05, a, b),
    high: 100 * betaQuantile(0.95, a, b),
    sd: (100 * Math.sqrt((a * b) / (total * total * (total + 1)))),
    weight: trials / (trials + strength),
  };
}

/** Normal-normal: the reference spread as the prior, one round's spread over the rounds as the noise. */
function rateEstimate(value: number, rounds: number, prior: Prior): Estimate | null {
  const round = prior.roundStddev ?? 0;
  if (!(rounds > 0) || !(prior.priorStddev > 0) || !(round > 0)) return null;
  const priorPrecision = 1 / (prior.priorStddev * prior.priorStddev);
  const dataPrecision = rounds / (round * round);
  const precision = priorPrecision + dataPrecision;
  const shrunk = (prior.mean * priorPrecision + value * dataPrecision) / precision;
  const sd = 1 / Math.sqrt(precision);
  return { raw: value, shrunk, low: shrunk - Z90 * sd, high: shrunk + Z90 * sd, sd, weight: dataPrecision / precision };
}

/** The shrunk estimate and its interval; null when the evidence or the prior cannot carry one. */
export function estimate(evidence: Evidence, prior: Prior): Estimate | null {
  if (!Number.isFinite(prior.mean)) return null;
  return evidence.kind === 'share' ? shareEstimate(evidence.successes, evidence.trials, prior) : rateEstimate(evidence.value, evidence.rounds, prior);
}

/** A ratio of two estimates, such as K/D, by the delta method on the logarithm. */
export function ratioEstimate(top: Estimate, bottom: Estimate): Estimate | null {
  if (!(top.shrunk > 0) || !(bottom.shrunk > 0) || !(bottom.raw > 0)) return null;
  const shrunk = top.shrunk / bottom.shrunk;
  const logSd = Math.hypot(top.sd / top.shrunk, bottom.sd / bottom.shrunk);
  return {
    raw: top.raw / bottom.raw,
    shrunk,
    low: shrunk * Math.exp(-Z90 * logSd),
    high: shrunk * Math.exp(Z90 * logSd),
    sd: shrunk * logSd,
    weight: Math.min(top.weight, bottom.weight),
  };
}

/** True when the interval leaves the reference out, so the difference is more than noise. */
export function differsFrom(e: Estimate, reference: number): boolean {
  return reference < e.low || reference > e.high;
}

/** "Estimación ajustada 0,74; con un 90 % de confianza, entre 0,62 y 0,86." */
export function estimateText(e: Estimate, format: (value: number) => string): string {
  const weak = e.weight < WEAK_WEIGHT ? ' Poca muestra: pesa más la referencia que sus rondas.' : '';
  return `Estimación ajustada ${format(e.shrunk)}; con un 90 % de confianza, entre ${format(e.low)} y ${format(e.high)}.${weak}`;
}
