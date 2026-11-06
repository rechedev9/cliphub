import type { Leaderboard, LeaderboardRow } from '../api/baseline.ts';

/** A ranking row that knows whether it is a professional. */
export type VersusRow = LeaderboardRow & { readonly pro: boolean };

/**
 * The library's players and the pros on one metric, ranked together. A pro
 * the library also holds keeps the library's row, marked as a pro.
 */
export function versusRows(library: Leaderboard, pro: Leaderboard, limit: number): VersusRow[] {
  const pros = new Set(pro.players.map((row) => row.steamid64));
  const rows: VersusRow[] = library.players.map((row) => ({ ...row, pro: pros.has(row.steamid64) }));
  const seen = new Set(rows.map((row) => row.steamid64));
  for (const row of pro.players) if (!seen.has(row.steamid64)) rows.push({ ...row, pro: true });
  const sign = library.higher_is_better ? -1 : 1;
  rows.sort((a, b) => sign * (a.value - b.value) || b.rounds - a.rounds || a.steamid64.localeCompare(b.steamid64));
  return rows.slice(0, limit).map((row, index) => ({ ...row, rank: index + 1 }));
}
