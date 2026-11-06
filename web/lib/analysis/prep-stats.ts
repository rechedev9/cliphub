import type { Baseline } from '../api/baseline.ts';
import type { TeamPrep } from '../api/playbook.ts';
import type { ScoutReport } from '../api/scout.ts';
import { METRIC_INFO, baselineValue } from './reference.ts';
import { poolMatchups, type RatePool, type StatInput } from './library-pool.ts';

const pct = (value: number): string => `${Math.round(value)} %`;

/** Rounds a team rate needs before it earns a sentence. */
const MIN_TEAM_ROUNDS = 10;

export type PrepStatSources = {
  readonly prep: TeamPrep;
  /** The team's scout on the same map and dates: openings and per-player damage. */
  readonly scout: ScoutReport | null;
  /** Matchups pooled over the library's teams on the map. */
  readonly library: RatePool | null;
  readonly baseline: Baseline | null;
};

/** The comparisons a prep reads as sentences: openings and per-round numbers against the baseline, win rates against the library. */
export function prepStatInputs({ prep, scout, library, baseline }: PrepStatSources): StatInput[] {
  const out: StatInput[] = [];
  const tendencies = scout?.tendencies;
  if (tendencies) {
    // Every opening duel has a winner, so the library's team mean is 50 % by construction.
    const won = tendencies.openings.won;
    out.push({ id: 'opening', subject: 'La apertura es', value: won.pct, reference: 50, format: pct, percent: true, higherIsBetter: true, threshold: 5, n: won.total, minSample: MIN_TEAM_ROUNDS });
  }
  if (tendencies) {
    const sum = tendencies.players.reduce(
      (acc, p) => ({ kills: acc.kills + p.kills, deaths: acc.deaths + p.deaths, damage: acc.damage + p.damage, rounds: acc.rounds + p.rounds }),
      { kills: 0, deaths: 0, damage: 0, rounds: 0 },
    );
    const per = [
      { id: 'adr', subject: 'El ADR es', metric: 'adr', value: sum.damage, threshold: 5 },
      { id: 'kpr', subject: 'Las bajas por ronda son', metric: 'kpr', value: sum.kills, threshold: 0.05 },
      { id: 'dpr', subject: 'Las muertes por ronda son', metric: 'dpr', value: sum.deaths, threshold: 0.05 },
    ] as const;
    for (const entry of per) {
      const reference = baselineValue(baseline, entry.metric);
      if (reference === null || sum.rounds === 0) continue;
      const info = METRIC_INFO[entry.metric];
      out.push({
        id: entry.id,
        subject: entry.subject,
        value: entry.value / sum.rounds,
        reference,
        format: info.format,
        percent: false,
        higherIsBetter: info.higherIsBetter,
        threshold: entry.threshold,
        n: tendencies.round_count,
        minSample: MIN_TEAM_ROUNDS,
      });
    }
  }
  if (library) {
    const own = poolMatchups([prep]);
    const rates = [
      { key: 'CT', subject: 'Gana en CT el' },
      { key: 'T', subject: 'Gana en T el' },
      { key: 'buy|pistol', subject: 'Gana las pistolas el' },
      { key: 'buy|eco', subject: 'Gana sus ecos el' },
      { key: 'buy|force', subject: 'Gana sus force el' },
      { key: 'buy|full', subject: 'Con compra completa gana el' },
    ];
    for (const rate of rates) {
      const value = own.pct(rate.key);
      const reference = library.pct(rate.key, MIN_TEAM_ROUNDS);
      if (value === null || reference === null) continue;
      out.push({ id: rate.key, subject: rate.subject, value, reference, format: pct, percent: true, higherIsBetter: true, threshold: 6, n: own.total(rate.key), minSample: 6 });
    }
  }
  return out;
}
