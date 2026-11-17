import type { DemoPlayer } from '../api/types.ts';

type Tally = Pick<DemoPlayer, 'kills' | 'deaths' | 'assists' | 'adr'>;

/** True for a slot that never fought: a coach stays on the team all match without a kill, death or damage. */
export function neverFought(player: Tally): boolean {
  return player.kills === 0 && player.deaths === 0 && player.assists === 0 && player.adr === 0;
}

/** The roster the picker offers: everyone who fought. A roster where nobody did stays whole. */
export function pickablePlayers<T extends Tally>(players: readonly T[]): T[] {
  const fought = players.filter((player) => !neverFought(player));
  return fought.length > 0 ? fought : [...players];
}
