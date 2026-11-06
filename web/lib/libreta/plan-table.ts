import type { LibraryPlay } from '../api/stratbook.ts';
import type { TacticalSide } from '../api/tactical.ts';
import { playKinds } from '../stratbook/plays.ts';

/** Every team, or plays without one: the Tabla's team choice besides a team id. */
export const PLAN_ALL_TEAMS = 'all';
export const PLAN_NO_TEAM = 'none';

export type PlanScope = { team: string; map: string; side: TacticalSide };
export type PlanColumn = { kind: string; plays: LibraryPlay[] };

function inScope(play: LibraryPlay, scope: PlanScope): boolean {
  if (play.status === 'archived' || play.map !== scope.map || play.side !== scope.side) return false;
  if (scope.team === PLAN_ALL_TEAMS) return true;
  if (scope.team === PLAN_NO_TEAM) return play.team_id === null;
  return play.team_id === scope.team;
}

/**
 * One column per round pattern of the side, each with its plays by title.
 * "Sin forma" only shows when a play has it, since nobody plans a round without one.
 */
export function planColumns(plays: readonly LibraryPlay[], scope: PlanScope): PlanColumn[] {
  const chosen = plays.filter((play) => inScope(play, scope)).sort((a, b) => a.title.localeCompare(b.title, 'es'));
  return playKinds(scope.side)
    .map((kind) => ({ kind, plays: chosen.filter((play) => play.kind === kind) }))
    .filter((column) => column.kind !== 'unknown' || column.plays.length > 0);
}

/** The map with most plays, so the Tabla opens where the work is. */
export function busiestMap(plays: readonly LibraryPlay[], fallback: string): string {
  const counts = new Map<string, number>();
  for (const play of plays) if (play.status !== 'archived') counts.set(play.map, (counts.get(play.map) ?? 0) + 1);
  let best = fallback;
  let most = 0;
  for (const [map, count] of counts) {
    if (count > most || (count === most && map < best)) {
      best = map;
      most = count;
    }
  }
  return best;
}
