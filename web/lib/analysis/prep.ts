import type { PrepMatchup, PrepTempo, PrepWindow } from '../api/playbook.ts';
import { roundClock } from '../replay/tramo.ts';
import { TACTICAL_BUY_TYPE_ORDER, type TacticalBuyType, type TacticalSide } from '../api/tactical.ts';

/** A window on the round clock, as the Replay's tramo reads: "1:55 a 1:00", or "0:40 al final" for the open last one. */
export function prepWindowLabel(window: PrepWindow): string {
  if (window.to_seconds === 0) return `${roundClock(window.from_seconds)} al final`;
  return `${roundClock(window.from_seconds)} a ${roundClock(window.to_seconds)}`;
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
  /** World position where the player stood there, or the callout's centre for a spot without one. */
  readonly x: number;
  readonly y: number;
  readonly pct: number;
  /** The player's most frequent spot in the window; the others are drawn smaller. */
  readonly main: boolean;
  /** Place among the markers drawn on one point, to fan them out. */
  readonly slot: number;
  readonly slots: number;
};

type WindowSpot = {
  readonly callout: string;
  readonly share: { readonly pct: number };
  readonly pos?: readonly [number, number, number];
  readonly floor?: string;
};
type WindowSpots = { readonly spots: readonly WindowSpot[] };

/** Spots under this share of the player's rounds stay in the list, off the radar. */
const MIN_MARKER_PCT = 20;
const MAX_MARKERS_PER_PLAYER = 2;
/** Markers closer than this, in world units, share a point and fan out. */
const SAME_POINT = 120;

/** The floor a spot is drawn on: "lower" for the lower radar of a two-level map. */
export function spotLevel(spot: Pick<WindowSpot, 'floor'>): 'default' | 'lower' {
  return spot.floor === 'lower' ? 'lower' : 'default';
}

/**
 * Where each CT player stands in one window of one floor, as markers at the
 * spot's real position: the main spot always, a second one when it holds 20 %
 * or more. A spot with no position falls back to the callout's centre, and
 * one the geometry does not know either is left off.
 */
export function windowMarkers(
  players: readonly { readonly windows: readonly WindowSpots[] }[],
  options: { window: number; centers: ReadonlyMap<string, readonly [number, number]>; level?: 'default' | 'lower' },
): SpotMarker[] {
  const level = options.level ?? 'default';
  const raw: Omit<SpotMarker, 'slot' | 'slots'>[] = [];
  players.forEach((player, index) => {
    const spots = player.windows[options.window]?.spots ?? [];
    spots
      .filter((spot, at) => at === 0 || spot.share.pct >= MIN_MARKER_PCT)
      .slice(0, MAX_MARKERS_PER_PLAYER)
      .forEach((spot, at) => {
        if (spotLevel(spot) !== level) return;
        const point = spot.pos ?? options.centers.get(spot.callout);
        if (point) raw.push({ player: index, callout: spot.callout, x: point[0], y: point[1], pct: spot.share.pct, main: at === 0 });
      });
  });
  const anchors: { x: number; y: number; count: number }[] = [];
  const group = raw.map((marker) => {
    let at = anchors.findIndex((anchor) => Math.hypot(anchor.x - marker.x, anchor.y - marker.y) < SAME_POINT);
    if (at < 0) at = anchors.push({ x: marker.x, y: marker.y, count: 0 }) - 1;
    const anchor = anchors[at];
    const slot = anchor ? anchor.count : 0;
    if (anchor) anchor.count += 1;
    return { at, slot };
  });
  return raw.map((marker, index) => {
    const entry = group[index];
    return { ...marker, slot: entry?.slot ?? 0, slots: anchors[entry?.at ?? 0]?.count ?? 1 };
  });
}
