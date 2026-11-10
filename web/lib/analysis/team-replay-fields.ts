import type { ScopeField } from './scope.ts';

/** The filters bar of a team's Replay, in the order it shows them; every one is applied by the server. */
export const TEAM_REPLAY_FIELDS: readonly ScopeField[] = [
  'lineup', 'side', 'period', 'limit', 'buy', 'opponentBuy', 'equip', 'opponentEquip', 'awp', 'opponentAwp', 'weapon', 'opponentWeapon',
  'advantage', 'site', 'outcome', 'endReason', 'postPlant', 'phase', 'secondRound', 'tactic', 'rounds', 'opponent', 'opponentTop', 'whose',
];

/** The match counts the Replay offers; with none in the URL it reads the server's cap. */
export const TEAM_REPLAY_LIMITS = { options: [3, 5, 10, 20, 50], fallback: 50 } as const;

/** The same filters with the map choice, for the stack that floats over the 2D replay of a team. */
export const TEAM_REPLAY_STACK_FIELDS: readonly ScopeField[] = ['lineup', 'map', ...TEAM_REPLAY_FIELDS.filter((field) => field !== 'lineup')];
