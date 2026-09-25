import type { FaceitFollowedPlayer, FaceitPlayer } from './api/faceit.ts';

export type FollowedPlayersState = {
  players: FaceitFollowedPlayer[];
  selectedID: string | null;
};

type Action =
  | { type: 'listed'; players: FaceitFollowedPlayer[] }
  | { type: 'selected'; id: string }
  | { type: 'followed'; player: FaceitFollowedPlayer }
  | { type: 'unfollowed'; id: string }
  | { type: 'profile'; player: FaceitPlayer };

/** The Players rail lists: the user's own follows, then one tab per seeded zone roster. */
export const PLAYER_TABS = [
  { id: 'custom', label: 'Custom' },
  { id: 'cis', label: 'CIS' },
  { id: 'latam', label: 'LATAM' },
] as const;

export type PlayerTab = (typeof PLAYER_TABS)[number]['id'];

/** A followed zone player shows in both Custom and their zone. */
export function inPlayerTab(player: FaceitFollowedPlayer, tab: PlayerTab): boolean {
  return tab === 'custom' ? player.seeded !== true : player.zone === tab;
}

/** Open on the user's own list when they follow anyone, otherwise on the first zone with players. */
export function defaultPlayerTab(players: FaceitFollowedPlayer[]): PlayerTab {
  return PLAYER_TABS.find((tab) => players.some((player) => inPlayerTab(player, tab.id)))?.id ?? 'custom';
}

export function followedPlayersReducer(state: FollowedPlayersState, action: Action): FollowedPlayersState {
  switch (action.type) {
    case 'listed':
      return { players: action.players, selectedID: action.players.some((player) => player.id === state.selectedID)
        ? state.selectedID : action.players[0]?.id ?? null };
    case 'selected':
      return state.players.some((player) => player.id === action.id) ? { ...state, selectedID: action.id } : state;
    case 'followed': {
      // The follow response carries no zone; keep the seeded row's so the player stays in that tab too.
      const zone = state.players.find((player) => player.id === action.player.id)?.zone;
      const followed = zone === undefined ? action.player : { ...action.player, zone };
      return { players: [followed, ...state.players.filter((player) => player.id !== action.player.id)], selectedID: followed.id };
    }
    case 'unfollowed': {
      // An unfollowed zone player goes back to being a seeded row of that zone, as the service lists them.
      const current = state.players.find((player) => player.id === action.id);
      if (current?.zone !== undefined && current.seeded !== true) {
        return { ...state, players: state.players.map((player) => player.id === action.id ? { ...player, seeded: true } : player) };
      }
      const players = state.players.filter((player) => player.id !== action.id);
      return { players, selectedID: state.selectedID === action.id ? players[0]?.id ?? null : state.selectedID };
    }
    case 'profile':
      // A live profile knows nothing about the roster, so it must not clear the row's list membership.
      return { ...state, players: state.players.map((player) => player.id === action.player.id
        ? { ...player, ...action.player, seeded: player.seeded, zone: player.zone } : player) };
  }
}
