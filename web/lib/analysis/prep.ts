import type { PrepMatchup, PrepTempo, PrepWindow } from '../api/playbook.ts';
import { TACTICAL_BUY_TYPE_ORDER, type TacticalBuyType, type TacticalSide } from '../api/tactical.ts';

/** "0-20 s", or "60 s o más" for the open last window. */
export function prepWindowLabel(window: PrepWindow): string {
  if (window.to_seconds === 0) return `${window.from_seconds} s o más`;
  return `${window.from_seconds}-${window.to_seconds} s`;
}

/** Buys in economic order that appear in the rows, `unknown` left out. */
function buysIn(values: readonly TacticalBuyType[]): TacticalBuyType[] {
  const present = new Set(values);
  return TACTICAL_BUY_TYPE_ORDER.filter((buy) => buy !== 'unknown' && present.has(buy));
}

export type MatchupGrid = {
  readonly buys: readonly TacticalBuyType[];
  readonly opponentBuys: readonly TacticalBuyType[];
  /** Keyed `buy|opponent_buy`. */
  readonly cells: ReadonlyMap<string, PrepMatchup>;
  /** Won over played across every cell of the side, the table's baseline; null for no rounds. */
  readonly baselinePct: number | null;
};

export function matchupKey(buy: TacticalBuyType, opponentBuy: TacticalBuyType): string {
  return `${buy}|${opponentBuy}`;
}

/** One side's buy matchups laid out as a grid: the team's buy per row, the rival's per column. */
export function matchupGrid(matchups: readonly PrepMatchup[], side: TacticalSide): MatchupGrid {
  const rows = matchups.filter((row) => row.side === side && row.buy !== 'unknown' && row.opponent_buy !== 'unknown');
  const cells = new Map(rows.map((row) => [matchupKey(row.buy, row.opponent_buy), row]));
  const played = rows.reduce((sum, row) => sum + row.win_rate.total, 0);
  const won = rows.reduce((sum, row) => sum + row.win_rate.count, 0);
  return {
    buys: buysIn(rows.map((row) => row.buy)),
    opponentBuys: buysIn(rows.map((row) => row.opponent_buy)),
    cells,
    baselinePct: played > 0 ? (won / played) * 100 : null,
  };
}

/** One side's tempo rows in economic order, rows with no sample left out. */
export function tempoRows(tempo: readonly PrepTempo[], side: TacticalSide): PrepTempo[] {
  const order = (buy: TacticalBuyType): number => TACTICAL_BUY_TYPE_ORDER.indexOf(buy);
  return tempo
    .filter((row) => row.side === side && row.buy !== 'unknown' && row.tempo.samples > 0)
    .sort((a, b) => order(a.buy) - order(b.buy));
}

/** Seconds as "23 s", one decimal under ten. */
export function secondsLabel(seconds: number): string {
  return seconds < 10 ? `${seconds.toFixed(1).replace('.', ',')} s` : `${Math.round(seconds)} s`;
}
