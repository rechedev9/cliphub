import { jsonInit, request } from './library.ts';

/** Browser-side contract of GET and PUT /pinned-players (`library.PinnedPlayer`); field names are the Go JSON names. */

export type PinnedPlayerEntry = { steamid64: string; name: string };

export const PINNED_PLAYERS_KEYS = ['players'] as const;
export const PINNED_PLAYERS_FIELDS = ['players'] as const;

const STEAM_ID64 = /^[0-9]{17}$/;

/** Studio refuses an id that is not a SteamID64, so a stale local pin is left out of a save. */
export function savablePins(pins: readonly PinnedPlayerEntry[]): PinnedPlayerEntry[] {
  return pins.filter((pin) => STEAM_ID64.test(pin.steamid64));
}

/** The Ranking's pinned players in table order, kept by Studio so an assistant reads the same list. */
export async function getPinnedPlayers(): Promise<PinnedPlayerEntry[]> {
  const body = await request<{ players: PinnedPlayerEntry[] }>('/pinned-players');
  return body.players;
}

/** Replaces the whole pinned list. */
export async function setPinnedPlayers(pins: readonly PinnedPlayerEntry[]): Promise<PinnedPlayerEntry[]> {
  const body = await request<{ players: PinnedPlayerEntry[] }>('/pinned-players', jsonInit('PUT', { players: savablePins(pins) }));
  return body.players;
}
