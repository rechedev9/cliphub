import type { TacticalDocument, TacticalTeam } from './tactical.ts';

/**
 * A team's player slots. Go encodes `Team.Slots` ([]uint8) as a base64 string,
 * so the slots are read back from the team's SteamIDs and the document's players.
 */
export function teamSlots(team: Pick<TacticalTeam, 'slots' | 'steamids'>, players: TacticalDocument['players']): number[] {
  const ids = new Set(team.steamids ?? []);
  const fromIds = players.filter((player) => ids.has(player.steamid64)).map((player) => player.slot);
  if (fromIds.length > 0 || !Array.isArray(team.slots)) return fromIds;
  return team.slots;
}

/** The document with each team's slots as numbers, the shape every reader expects. */
export function withTeamSlots(doc: TacticalDocument): TacticalDocument {
  return { ...doc, teams: doc.teams.map((team) => ({ ...team, slots: teamSlots(team, doc.players) })) };
}
