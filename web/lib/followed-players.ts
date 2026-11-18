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

/** The rail of Jugadores FACEIT: the user's own follows, and the ranking of the chosen country, region or zone. */
export const PLAYER_TABS = [
  { id: 'seguidos', label: 'Seguidos' },
  { id: 'ranking', label: 'Ranking' },
] as const;

export type PlayerTab = (typeof PLAYER_TABS)[number]['id'];

/** Open on the user's own list when they follow anyone, otherwise on the ranking. */
export function defaultPlayerTab(players: readonly FaceitFollowedPlayer[]): PlayerTab {
  return players.length > 0 ? 'seguidos' : 'ranking';
}

export function followedPlayersReducer(state: FollowedPlayersState, action: Action): FollowedPlayersState {
  switch (action.type) {
    case 'listed':
      // An own follow's listed row is the profile the service stored last, so it always wins.
      return { players: action.players, selectedID: action.players.some((player) => player.id === state.selectedID)
        ? state.selectedID : action.players[0]?.id ?? null };
    case 'selected':
      return state.players.some((player) => player.id === action.id) ? { ...state, selectedID: action.id } : state;
    case 'followed':
      return { players: [action.player, ...state.players.filter((player) => player.id !== action.player.id)], selectedID: action.player.id };
    case 'unfollowed': {
      const players = state.players.filter((player) => player.id !== action.id);
      return { players, selectedID: state.selectedID === action.id ? players[0]?.id ?? null : state.selectedID };
    }
    case 'profile':
      return { ...state, players: state.players.map((player) => player.id === action.player.id
        ? { ...player, ...action.player } : player) };
  }
}
