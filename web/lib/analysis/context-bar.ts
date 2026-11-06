import { SCOPE_PARAMS } from './scope.ts';

/**
 * The context bar's arithmetic: the last results shown as dots beside the
 * sample, and the link that swaps the entity while keeping tab and filters.
 */

export type ResultDot = 'win' | 'loss' | 'draw';

export type DatedResult = { readonly playedAt: string; readonly outcome: ResultDot | null };

/** The `max` most recent known results, oldest first so they read left to right like a timeline. */
export function lastResults(rows: readonly DatedResult[], max: number): ResultDot[] {
  const known: { at: number; outcome: ResultDot }[] = [];
  for (const row of rows) {
    if (row.outcome === null) continue;
    const at = Date.parse(row.playedAt);
    known.push({ at: Number.isNaN(at) ? 0 : at, outcome: row.outcome });
  }
  known.sort((a, b) => a.at - b.at);
  return known.slice(Math.max(0, known.length - max)).map((entry) => entry.outcome);
}

/** A match's result from its two scores; null when neither side scored, which means it was not read. */
export function scoreOutcome(own: number, rival: number): ResultDot | null {
  if (own === 0 && rival === 0) return null;
  if (own > rival) return 'win';
  return own < rival ? 'loss' : 'draw';
}

/** "3-1" as wins and losses, with draws only when there are any. */
export function resultRecord(results: readonly ResultDot[]): string {
  const wins = results.filter((r) => r === 'win').length;
  const losses = results.filter((r) => r === 'loss').length;
  const draws = results.length - wins - losses;
  return draws > 0 ? `${wins}-${losses}-${draws}` : `${wins}-${losses}`;
}

/** The query param that keeps the open tab of a team or player page. */
export const TAB_PARAM = 'tab';

/** The tab named in the query when the page has it, else the fallback. */
export function tabFromQuery(query: Pick<URLSearchParams, 'get'>, tabs: readonly string[], fallback: string): string {
  const named = query.get(TAB_PARAM);
  return named !== null && tabs.includes(named) ? named : fallback;
}

/** The query with the tab written over it; the default tab is left out. */
export function writeTab(current: string, tab: string, fallback: string): string {
  const params = new URLSearchParams(current);
  if (tab === fallback) params.delete(TAB_PARAM);
  else params.set(TAB_PARAM, tab);
  return params.toString();
}

/**
 * The link to another team or player with the same tab and filters. A rival
 * filter belongs to the entity it was set on, so it is dropped.
 */
export function switchEntityHref(base: string, current: string): string {
  const params = new URLSearchParams(current);
  params.delete(SCOPE_PARAMS.opponent);
  const search = params.toString();
  return search === '' ? base : `${base}?${search}`;
}
