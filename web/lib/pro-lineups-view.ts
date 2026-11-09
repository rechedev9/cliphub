import { filterProLineups, PRO_KIND_LABEL, PRO_LINEUP_KINDS, type ProLineup, type ProLineupKind } from './api/pro-lineups.ts';
import { RADAR_LEVELS, type RadarLevel, type TacticalSide } from './api/tactical.ts';

/**
 * How Lineups pro is browsed: lineups grouped by where they land, the way
 * players look for them ("a smoke for Window"), and the explorer state kept in
 * the URL so going back from a lineup returns to the same view.
 */

/** Landings closer than this, in world units, of the same grenade and callout, are one spot. */
export const SPOT_REACH = 250;

/** One place lineups land: every lineup of one grenade that lands there, most thrown first. */
export type ProSpot = {
  /** The most thrown lineup's id: stable while the library keeps that lineup. */
  id: string;
  kind: ProLineupKind;
  place: string;
  /** Where the most thrown lineup lands. */
  landing: [number, number, number];
  lineups: ProLineup[];
  /** Recent throws of all its lineups. */
  throws: number;
};

/**
 * Groups lineups by landing: same grenade, same callout, within `SPOT_REACH`
 * of the spot's most thrown landing. Built once from the whole map so a spot
 * stays put while filters only change which of its lineups show.
 */
export function landingSpots(lineups: readonly ProLineup[]): ProSpot[] {
  const byUse = [...lineups].sort((a, b) => b.throws.recent - a.throws.recent);
  const spots: ProSpot[] = [];
  for (const lineup of byUse) {
    const spot = spots.find(
      (s) =>
        s.kind === lineup.kind &&
        s.place === lineup.land_place &&
        Math.hypot(s.landing[0] - lineup.landing[0], s.landing[1] - lineup.landing[1]) < SPOT_REACH,
    );
    if (spot === undefined) {
      spots.push({ id: lineup.id, kind: lineup.kind, place: lineup.land_place, landing: lineup.landing, lineups: [lineup], throws: lineup.throws.recent });
    } else {
      spot.lineups.push(lineup);
      spot.throws += lineup.throws.recent;
    }
  }
  return spots.sort((a, b) => b.throws - a.throws);
}

/** The spots with the lineups a filter keeps; spots left empty go. */
export function visibleSpots(spots: readonly ProSpot[], keep: ReadonlySet<string>): ProSpot[] {
  const out: ProSpot[] = [];
  for (const spot of spots) {
    const lineups = spot.lineups.filter((l) => keep.has(l.id));
    if (lineups.length > 0) out.push({ ...spot, lineups, throws: lineups.reduce((n, l) => n + l.throws.recent, 0) });
  }
  return out.sort((a, b) => b.throws - a.throws);
}

/** The spot a lineup lands at. */
export function spotOf(spots: readonly ProSpot[], lineupId: string): ProSpot | undefined {
  return spots.find((s) => s.lineups.some((l) => l.id === lineupId));
}

/** "Humo a Window": what a spot is called in lists and headings. */
export function spotLabel(spot: Pick<ProSpot, 'kind' | 'place'>): string {
  return `${PRO_KIND_LABEL[spot.kind]} a ${spot.place || 'zona sin nombre'}`;
}

/**
 * A lineup title without the server's " · variante N": the number only
 * separates titles in the library, while the page tells variants apart by where
 * they are thrown from and how.
 */
export function plainTitle(title: string): string {
  return title.replace(/ · variante \d+$/, '');
}

export type ExplorerView = 'lineups' | 'combos';
export type ExplorerKind = ProLineupKind | 'all';

/** Everything the map explorer shows, as kept in its URL. */
export type ExplorerQuery = {
  view: ExplorerView;
  kind: ExplorerKind;
  side: TacticalSide | '';
  insta: boolean;
  video: boolean;
  text: string;
  /** A lineup id of the open spot, '' for the spot list. */
  spot: string;
  /** The lineup previewed inside the spot. */
  lineup: string;
  combo: string;
  level: RadarLevel;
};

/** Smokes first, as players look for them most. */
export const DEFAULT_QUERY: ExplorerQuery = {
  view: 'lineups',
  kind: 'smoke',
  side: '',
  insta: false,
  video: false,
  text: '',
  spot: '',
  lineup: '',
  combo: '',
  level: RADAR_LEVELS.default,
};

function oneOf<T extends string>(value: string | null, allowed: readonly T[], fallback: T): T {
  return allowed.find((option) => option === value) ?? fallback;
}

export function readExplorerQuery(search: string): ExplorerQuery {
  const params = new URLSearchParams(search);
  return {
    view: oneOf(params.get('vista'), ['lineups', 'combos'], DEFAULT_QUERY.view),
    kind: oneOf<ExplorerKind>(params.get('granada'), [...PRO_LINEUP_KINDS, 'all'], DEFAULT_QUERY.kind),
    side: oneOf<TacticalSide | ''>(params.get('lado'), ['T', 'CT'], ''),
    insta: params.get('insta') === '1',
    video: params.get('video') === '1',
    text: params.get('q') ?? '',
    spot: params.get('destino') ?? '',
    lineup: params.get('lineup') ?? '',
    combo: params.get('combo') ?? '',
    level: oneOf<RadarLevel>(params.get('piso'), [RADAR_LEVELS.default, RADAR_LEVELS.lower], DEFAULT_QUERY.level),
  };
}

/** The query string for a state, defaults left out; '' when everything is default. */
export function writeExplorerQuery(query: ExplorerQuery): string {
  const params = new URLSearchParams();
  if (query.view !== DEFAULT_QUERY.view) params.set('vista', query.view);
  if (query.kind !== DEFAULT_QUERY.kind) params.set('granada', query.kind);
  if (query.side !== '') params.set('lado', query.side);
  if (query.insta) params.set('insta', '1');
  if (query.video) params.set('video', '1');
  if (query.text !== '') params.set('q', query.text);
  if (query.spot !== '') params.set('destino', query.spot);
  if (query.lineup !== '') params.set('lineup', query.lineup);
  if (query.combo !== '') params.set('combo', query.combo);
  if (query.level !== DEFAULT_QUERY.level) params.set('piso', query.level);
  return params.toString();
}

/** The lineups the explorer's filters keep, before the spot or level narrows them. */
export function explorerLineups(lineups: readonly ProLineup[], query: ExplorerQuery, kind: ExplorerKind = query.kind): ProLineup[] {
  const kinds = new Set<ProLineupKind>(kind === 'all' ? PRO_LINEUP_KINDS : [kind]);
  return filterProLineups(lineups, { kinds, side: query.side, insta: query.insta, video: query.video, text: query.text });
}

/** The explorer link that opens a lineup's spot with it previewed, for the way back from its page. */
export function spotHref(map: string, lineup: Pick<ProLineup, 'id' | 'kind'>): string {
  const search = writeExplorerQuery({ ...DEFAULT_QUERY, kind: lineup.kind, spot: lineup.id, lineup: lineup.id });
  return `/lineups/${map}?${search}`;
}
