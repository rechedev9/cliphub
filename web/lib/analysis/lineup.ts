import type { TeamLineups } from '../api/team-insights.ts';
import { prettyMapName } from '../format.ts';
import type { MapCounts } from './map-counts.ts';
import { lineupOf, type ScopeLineup } from './scope.ts';

/**
 * The lineup picker's arithmetic: which players a lineup checks, which
 * SteamID64s it sends, and the lineup a set of checked boxes stands for.
 * The latest lineup is empty when the latest match names none of the team's players; it then counts everyone.
 */

function latestIds(lineups: TeamLineups): string[] {
  return lineups.latest.map((player) => player.steamid64);
}

function everyId(lineups: TeamLineups): string[] {
  return [...latestIds(lineups), ...lineups.others.map((player) => player.steamid64)];
}

function countsEveryone(lineup: ScopeLineup, lineups: TeamLineups): boolean {
  return lineup.kind === 'all' || (lineup.kind === 'latest' && lineups.latest.length === 0);
}

/** The SteamID64s the server filters by; empty keeps every round, as it does while the groups are unknown. */
export function lineupSteamIds(lineup: ScopeLineup, lineups: TeamLineups | null): readonly string[] {
  if (lineup.kind === 'players') return lineup.steamIds;
  if (lineup.kind === 'all' || lineups === null) return [];
  return [...latestIds(lineups)].sort();
}

/** The roster players whose boxes are checked. */
export function checkedPlayers(lineup: ScopeLineup, lineups: TeamLineups): ReadonlySet<string> {
  if (lineup.kind === 'players') return new Set(lineup.steamIds);
  return new Set(countsEveryone(lineup, lineups) ? everyId(lineups) : latestIds(lineups));
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

export type LineupToggle = { lineup: ScopeLineup; lineups: TeamLineups; steamId: string };

/** The lineup with one player's box flipped; the last checked box stays, since an empty lineup counts nothing. */
export function toggleLineupPlayer({ lineup, lineups, steamId }: LineupToggle): ScopeLineup {
  const checked = new Set(checkedPlayers(lineup, lineups));
  if (checked.has(steamId)) {
    if (checked.size === 1) return lineup;
    checked.delete(steamId);
  } else {
    checked.add(steamId);
  }
  return lineupFromChecked(checked, lineups);
}

/** What the picker's button says: how many of the team's players count, as in "5 de 7 jugadores". */
export function lineupLabel(lineup: ScopeLineup, lineups: TeamLineups): string {
  const every = everyId(lineups);
  const checked = checkedPlayers(lineup, lineups);
  const count = every.filter((steamId) => checked.has(steamId)).length;
  return `${count} de ${every.length} ${every.length === 1 ? 'jugador' : 'jugadores'}`;
}

/** Which lineup counts: "Última alineación", "Todos los jugadores" or "Alineación elegida". */
export function lineupMode(lineup: ScopeLineup, lineups: TeamLineups): string {
  if (countsEveryone(lineup, lineups)) return 'Todos los jugadores';
  return lineup.kind === 'latest' ? 'Última alineación' : 'Alineación elegida';
}

export type LatestLineupGap = {
  lineup: ScopeLineup;
  lineups: TeamLineups | null;
  map: string;
  /** Matches per map under the view's scope, lineup included; null while counting. */
  mapCounts: MapCounts | null;
};

/** The default lineup counts no match on the chosen map, as when a new player joined after the team last played it. */
export function latestLineupMissesMap({ lineup, lineups, map, mapCounts }: LatestLineupGap): boolean {
  if (lineup.kind !== 'latest' || lineups === null || lineups.latest.length === 0) return false;
  return map !== '' && mapCounts !== null && (mapCounts[map] ?? 0) === 0;
}

export type LineupForMap = {
  /** SteamID64s of the lineup the coach has in scope. */
  chosen: readonly string[];
  map: string;
  /** Matches per map with the chosen lineup; null while counting. */
  latestCounts: MapCounts | null;
  /** Matches per map with every player, read only when the chosen lineup is the default and misses the map. */
  everyoneCounts: MapCounts | null;
  misses: boolean;
};

export type MapLineup = {
  lineup: readonly string[];
  mapCounts: MapCounts | null;
  /** Matches the view shows with every player because the default lineup never played the map; 0 otherwise. */
  fallback: number;
};

const EVERYONE: readonly string[] = [];

/** On a map the latest lineup never played, the default reads every player, so a team's matches there still show. */
export function lineupForMap({ chosen, map, latestCounts, everyoneCounts, misses }: LineupForMap): MapLineup {
  const fallback = misses ? (everyoneCounts?.[map] ?? 0) : 0;
  if (fallback === 0 || latestCounts === null) return { lineup: chosen, mapCounts: latestCounts, fallback: 0 };
  return { lineup: EVERYONE, mapCounts: { ...latestCounts, [map]: fallback }, fallback };
}

/** Why the view reads every player on this map: "La última alineación no ha jugado en Cache. …". */
export function latestLineupGapText(map: string, matches: number): string {
  const shown = matches === 1 ? 'Se ve 1 partida' : `Se ven las ${matches} partidas`;
  return `La última alineación no ha jugado en ${prettyMapName(map)}. ${shown} con todos los jugadores.`;
}
