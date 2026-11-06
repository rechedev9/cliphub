import type { TacticalBuyType, TacticalRate, TacticalSide } from '../api/tactical.ts';
import type { PlaybookCategoryName } from '../api/playbook.ts';
import type { BombsiteMap } from '../api/team-insights.ts';

/**
 * The library as a reference for one team: an adaptive player minimum for the
 * baseline, and pools keyed like a team table, filled from a team's own rows
 * or from the library norms, so both read with one definition.
 */

export const MIN_ROUNDS_CAP = 50;
export const MIN_ROUNDS_FLOOR = 15;

/** The median player's rounds, kept between 15 and 50, so a small library still has a reference. */
export function adaptiveMinRounds(rounds: readonly number[]): number {
  const sorted = rounds.filter((value) => value > 0).sort((a, b) => a - b);
  if (sorted.length === 0) return MIN_ROUNDS_CAP;
  const middle = sorted[Math.floor((sorted.length - 1) / 2)] ?? MIN_ROUNDS_CAP;
  return Math.max(MIN_ROUNDS_FLOOR, Math.min(MIN_ROUNDS_CAP, Math.round(middle)));
}

export type RateSum = { count: number; total: number };

/** Sums of rates by key; `pooledPct` reads one back as a percent. */
export class RatePool {
  private readonly sums = new Map<string, RateSum>();

  add(key: string, rate: Pick<TacticalRate, 'count' | 'total'>): void {
    const sum = this.sums.get(key) ?? { count: 0, total: 0 };
    this.sums.set(key, { count: sum.count + rate.count, total: sum.total + rate.total });
  }

  /** The pooled percent, or null under `minTotal` samples. */
  pct(key: string, minTotal = 1): number | null {
    const sum = this.sums.get(key);
    return sum && sum.total >= minTotal ? (100 * sum.count) / sum.total : null;
  }

  total(key: string): number {
    return this.sums.get(key)?.total ?? 0;
  }
}

type RateOf = Pick<TacticalRate, 'count' | 'total'>;
type CategoryRows = { readonly win_rate: RateOf; readonly categories: readonly { readonly category: PlaybookCategoryName; readonly share: RateOf; readonly win_rate: RateOf }[] };
type MatchupRows = { readonly matchups: readonly { readonly side: TacticalSide; readonly buy: TacticalBuyType; readonly opponent_buy: TacticalBuyType; readonly win_rate: RateOf }[] };

/** Each category's use (`share`) and success (`win_rate`) pooled over the playbooks of one side. */
export function poolCategories(playbooks: readonly CategoryRows[]): RatePool {
  const pool = new RatePool();
  for (const playbook of playbooks) {
    pool.add('win', playbook.win_rate);
    for (const category of playbook.categories) {
      pool.add(`used|${category.category}`, category.share);
      pool.add(`won|${category.category}`, category.win_rate);
    }
  }
  return pool;
}

/** One side's categories from the library norms, keyed like `poolCategories`. */
export function normsCategories(norms: { readonly playbook: readonly (CategoryRows & { readonly side: TacticalSide })[] }, side: TacticalSide): RatePool {
  return poolCategories(norms.playbook.filter((entry) => entry.side === side));
}

/** "la partida de la biblioteca" or "las 12 partidas de la biblioteca". */
export function libraryMatchesText(matches: number): string {
  return matches === 1 ? 'la partida de la biblioteca' : `las ${matches} partidas de la biblioteca`;
}

export function categoryKey(kind: 'used' | 'won', category: PlaybookCategoryName): string {
  return `${kind}|${category}`;
}

/** Win rate per side and buy matchup, per side overall and per own buy, pooled over the sources. */
export function poolMatchups(preps: readonly MatchupRows[]): RatePool {
  const pool = new RatePool();
  for (const prep of preps) {
    for (const row of prep.matchups) {
      if (row.buy === 'unknown' || row.opponent_buy === 'unknown') continue;
      pool.add(`${row.side}|${row.buy}|${row.opponent_buy}`, row.win_rate);
      pool.add(`${row.side}|${row.opponent_buy}`, row.win_rate);
      pool.add(row.side, row.win_rate);
      pool.add(`buy|${row.buy}`, row.win_rate);
    }
  }
  return pool;
}

export const BOMBSITE_FIELDS = {
  ct: ['hit', 'held', 'plants_allowed', 'won', 'retakes_won'],
  t: ['attacks', 'plants', 'won', 'post_plant_won'],
} as const;
export type DefenseField = (typeof BOMBSITE_FIELDS.ct)[number];
export type AttackField = (typeof BOMBSITE_FIELDS.t)[number];

export function bombsiteKey(map: string, site: string, field: `ct.${DefenseField}` | `t.${AttackField}`): string {
  return `${map}|${site}|${field}`;
}

/** Every bombsite rate keyed by map and site. */
export function poolBombsites(maps: readonly BombsiteMap[]): RatePool {
  const pool = new RatePool();
  for (const entry of maps) {
    for (const site of entry.sites) {
      for (const field of BOMBSITE_FIELDS.ct) pool.add(bombsiteKey(entry.map, site.site, `ct.${field}`), site.ct[field]);
      for (const field of BOMBSITE_FIELDS.t) pool.add(bombsiteKey(entry.map, site.site, `t.${field}`), site.t[field]);
    }
  }
  return pool;
}

export type StatInput = {
  readonly id: string;
  /** The subject with its verb, such as "La apertura es" or "Gana en CT el". */
  readonly subject: string;
  readonly value: number;
  readonly reference: number;
  readonly format: (value: number) => string;
  /** True for percents: the gap reads in points. */
  readonly percent: boolean;
  readonly higherIsBetter: boolean;
  /** The smallest gap worth a sentence, in the value's own units. */
  readonly threshold: number;
  /** Samples behind the value; under `minSample` the stat is left out. */
  readonly n: number;
  readonly minSample: number;
};

export type SignificantStat = { readonly id: string; readonly text: string; readonly tone: 'better' | 'worse'; readonly weight: number };

/** "La apertura es 39 %, 6 puntos peor que la media", the largest gaps first. */
export function significantStats(inputs: readonly StatInput[], max = 6): SignificantStat[] {
  const out: SignificantStat[] = [];
  for (const input of inputs) {
    if (input.n < input.minSample || !Number.isFinite(input.value) || !Number.isFinite(input.reference)) continue;
    const delta = input.value - input.reference;
    if (Math.abs(delta) < input.threshold) continue;
    const better = input.higherIsBetter ? delta > 0 : delta < 0;
    const gap = input.percent ? `${Math.round(Math.abs(delta))} puntos` : input.format(Math.abs(delta));
    out.push({
      id: input.id,
      text: `${input.subject} ${input.format(input.value)}, ${gap} ${better ? 'mejor' : 'peor'} que la media`,
      tone: better ? 'better' : 'worse',
      weight: Math.abs(delta) / input.threshold,
    });
  }
  return out.sort((a, b) => b.weight - a.weight).slice(0, max);
}
