import type { PlayerMatchLine } from '../api/metrics.ts';
import type { BaselineMetric } from '../api/baseline.ts';

/** A player's numbers match by match, oldest first: the sparklines and the recent performance chart. */

export const TREND_METRICS = ['kd', 'kpr', 'adr', 'wpa'] as const;
export type TrendMetric = (typeof TREND_METRICS)[number];

export const TREND_LABEL: Readonly<Record<TrendMetric, string>> = {
  kd: 'K/D',
  kpr: 'Bajas por ronda',
  adr: 'ADR',
  wpa: 'WPA por ronda',
};

/** The reference metric a trend compares with; K/D has none. */
export const TREND_BASELINE: Readonly<Record<TrendMetric, BaselineMetric | null>> = {
  kd: null,
  kpr: 'kpr',
  adr: 'adr',
  wpa: 'wpa_per_round',
};

export type TrendPoint = { matchId: string; map: string; playedAt: string; value: number; rounds: number };

/** The player's own damage and rounds in one match, from the match detail; the metrics lines carry no damage. */
export type MatchDamage = { damage: number; rounds: number };

function value(metric: TrendMetric, line: PlayerMatchLine, damage: MatchDamage | undefined): number | null {
  if (line.rounds <= 0) return null;
  if (metric === 'kd') return line.deaths > 0 ? line.kills / line.deaths : line.kills;
  if (metric === 'kpr') return line.kills / line.rounds;
  if (metric === 'wpa') return line.wpa / line.rounds;
  return damage && damage.rounds > 0 ? damage.damage / damage.rounds : null;
}

/** One point per match with a value, oldest first; a match without damage drops out of ADR only. */
export function trendPoints(
  lines: readonly PlayerMatchLine[],
  metric: TrendMetric,
  damage: ReadonlyMap<string, MatchDamage> = new Map(),
): TrendPoint[] {
  const out: TrendPoint[] = [];
  for (const line of [...lines].sort((a, b) => Date.parse(a.played_at) - Date.parse(b.played_at))) {
    const v = value(metric, line, damage.get(line.match_id));
    if (v !== null) out.push({ matchId: line.match_id, map: line.map, playedAt: line.played_at, value: v, rounds: line.rounds });
  }
  return out;
}
