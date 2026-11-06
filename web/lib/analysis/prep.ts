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

export type LabelSide = 'right' | 'left' | 'above' | 'below';
type Box = { x: number; y: number; width: number; height: number };

function boxAt(dot: { x: number; y: number; width: number; height: number }, side: LabelSide, gap: number): Box {
  if (side === 'right') return { x: dot.x + gap, y: dot.y - dot.height / 2, width: dot.width, height: dot.height };
  if (side === 'left') return { x: dot.x - gap - dot.width, y: dot.y - dot.height / 2, width: dot.width, height: dot.height };
  if (side === 'above') return { x: dot.x - dot.width / 2, y: dot.y - gap - dot.height, width: dot.width, height: dot.height };
  return { x: dot.x - dot.width / 2, y: dot.y + gap, width: dot.width, height: dot.height };
}

function overlaps(a: Box, b: Box): boolean {
  return a.x < b.x + b.width && b.x < a.x + a.width && a.y < b.y + b.height && b.y < a.y + a.height;
}

/** A side for each dot's label so no two labels collide, greedy in order; right when every side collides. */
export function placeLabels(dots: readonly { x: number; y: number; width: number; height: number }[], gap: number): LabelSide[] {
  const placed: Box[] = [];
  return dots.map((dot) => {
    const side = (['right', 'left', 'above', 'below'] as const).find((option) => !placed.some((box) => overlaps(box, boxAt(dot, option, gap)))) ?? 'right';
    placed.push(boxAt(dot, side, gap));
    return side;
  });
}

export type SpotMarker = {
  readonly player: number;
  readonly callout: string;
  /** World position of the callout's centre. */
  readonly x: number;
  readonly y: number;
  readonly pct: number;
  /** The player's most frequent spot in the window; the others are drawn smaller. */
  readonly main: boolean;
  /** Place among the markers on the same callout, to fan them out. */
  readonly slot: number;
  readonly slots: number;
};

type WindowSpots = { readonly spots: readonly { readonly callout: string; readonly share: { readonly pct: number } }[] };

/** Spots under this share of the player's rounds stay in the list, off the radar. */
const MIN_MARKER_PCT = 20;
const MAX_MARKERS_PER_PLAYER = 2;

/**
 * Where each CT player stands in one window, as markers at the callouts'
 * centres: the main spot always, a second one when it holds 20 % or more.
 * A callout the geometry does not know is left off.
 */
export function windowMarkers(
  players: readonly { readonly windows: readonly WindowSpots[] }[],
  options: { window: number; centers: ReadonlyMap<string, readonly [number, number]> },
): SpotMarker[] {
  const raw: Omit<SpotMarker, 'slot' | 'slots'>[] = [];
  players.forEach((player, index) => {
    const spots = player.windows[options.window]?.spots ?? [];
    spots
      .filter((spot, at) => at === 0 || spot.share.pct >= MIN_MARKER_PCT)
      .slice(0, MAX_MARKERS_PER_PLAYER)
      .forEach((spot, at) => {
        const center = options.centers.get(spot.callout);
        if (center) raw.push({ player: index, callout: spot.callout, x: center[0], y: center[1], pct: spot.share.pct, main: at === 0 });
      });
  });
  const groups = new Map<string, number>();
  for (const marker of raw) groups.set(marker.callout, (groups.get(marker.callout) ?? 0) + 1);
  const seen = new Map<string, number>();
  return raw.map((marker) => {
    const slot = seen.get(marker.callout) ?? 0;
    seen.set(marker.callout, slot + 1);
    return { ...marker, slot, slots: groups.get(marker.callout) ?? 1 };
  });
}
