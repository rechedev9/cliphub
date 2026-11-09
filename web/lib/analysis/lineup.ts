import type { TeamLineups } from '../api/team-insights.ts';
import { lineupOf, type ScopeLineup } from './scope.ts';

/**
 * The lineup picker's arithmetic: which players a lineup checks, which
 * SteamID64s it sends, and the lineup a set of checked boxes stands for.
 */

function latestIds(lineups: TeamLineups): string[] {
  return lineups.latest.map((player) => player.steamid64);
}

function everyId(lineups: TeamLineups): string[] {
  return [...latestIds(lineups), ...lineups.others.map((player) => player.steamid64)];
}

/** The SteamID64s the server filters by; empty keeps every round, as it does while the groups are unknown. */
export function lineupSteamIds(lineup: ScopeLineup, lineups: TeamLineups | null): readonly string[] {
  if (lineup.kind === 'players') return lineup.steamIds;
  if (lineup.kind === 'all' || lineups === null) return [];
  return [...latestIds(lineups)].sort();
}

/** The roster players whose boxes are checked. */
export function checkedPlayers(lineup: ScopeLineup, lineups: TeamLineups): ReadonlySet<string> {
  if (lineup.kind === 'all') return new Set(everyId(lineups));
  if (lineup.kind === 'latest') return new Set(latestIds(lineups));
  return new Set(lineup.steamIds);
}

function sameSet(a: ReadonlySet<string>, b: readonly string[]): boolean {
  return a.size === b.length && b.every((steamId) => a.has(steamId));
}

/** The lineup checked boxes stand for: the latest or every player when they match one, else the set itself. */
export function lineupFromChecked(checked: ReadonlySet<string>, lineups: TeamLineups): ScopeLineup {
  if (lineups.latest.length > 0 && sameSet(checked, latestIds(lineups))) return { kind: 'latest' };
  if (sameSet(checked, everyId(lineups))) return { kind: 'all' };
  return lineupOf([...checked]);
}

/** The lineup with one player's box flipped; the last checked box stays, since an empty lineup counts nothing. */
export function toggleLineupPlayer(lineup: ScopeLineup, lineups: TeamLineups, steamId: string): ScopeLineup {
  const checked = new Set(checkedPlayers(lineup, lineups));
  if (checked.has(steamId)) {
    if (checked.size === 1) return lineup;
    checked.delete(steamId);
  } else {
    checked.add(steamId);
  }
  return lineupFromChecked(checked, lineups);
}

/** What the picker's button says: "Última alineación", "Todos los jugadores" or "3 jugadores". */
export function lineupLabel(lineup: ScopeLineup): string {
  if (lineup.kind === 'latest') return 'Última alineación';
  if (lineup.kind === 'all') return 'Todos los jugadores';
  const count = lineup.steamIds.length;
  return `${count} ${count === 1 ? 'jugador' : 'jugadores'}`;
}
