import type { ProLineup, ProLineupKind, ProLineupMap } from './api/pro-lineups.ts';
import type { TacticalSide } from './api/tactical.ts';

/**
 * The grenade meta of a map side: its pro lineups ranked by throws per 100
 * side rounds in a period, with the trend of the last 30 days against the 60
 * before. `pro_grenade_meta` in the MCP reads the same way.
 */

export type MetaPeriod = 'recent' | 'all';
export type MetaTrend = 'new' | 'rising' | 'falling' | 'steady';

export const META_TREND_LABEL: Record<MetaTrend, string> = {
  new: 'Entra en el meta',
  rising: 'Sube',
  falling: 'Baja',
  steady: 'Estable',
};

/** A rate rises or falls when it moved by half and by at least one throw per 100 rounds. */
const TREND_FACTOR = 1.5;
const TREND_MIN = 1;

/** The trend of throws per 100 side rounds now against before; null while the index has no trend. */
export function metaTrend(trend: readonly [number, number] | undefined): MetaTrend | null {
  if (trend === undefined) return null;
  const [now, before] = trend;
  if (before === 0 && now > 0) return 'new';
  if (now >= before * TREND_FACTOR && now - before >= TREND_MIN) return 'rising';
  if (now * TREND_FACTOR <= before && before - now >= TREND_MIN) return 'falling';
  return 'steady';
}

export type MetaRow = { lineup: ProLineup; rank: number; throws: number; per100: number; trend: MetaTrend | null };

/** The side's lineups thrown in the period, most per 100 side rounds first. */
export function grenadeMeta(
  data: Pick<ProLineupMap, 'lineups' | 'rounds_t' | 'rounds_ct'>,
  options: { side: TacticalSide; kinds: ReadonlySet<ProLineupKind>; period: MetaPeriod },
): MetaRow[] {
  const { side, kinds, period } = options;
  const rounds = (side === 'CT' ? data.rounds_ct : data.rounds_t)[period];
  const rows = data.lineups.flatMap((lineup) => {
    const throws = lineup.throws[period];
    if (lineup.side !== side || !kinds.has(lineup.kind) || throws === 0) return [];
    const per100 = rounds > 0 ? Math.round((1000 * throws) / rounds) / 10 : 0;
    return [{ lineup, rank: 0, throws, per100, trend: metaTrend(lineup.trend) }];
  });
  rows.sort((a, b) => b.per100 - a.per100 || b.throws - a.throws || a.lineup.id.localeCompare(b.lineup.id));
  return rows.map((row, index) => ({ ...row, rank: index + 1 }));
}
