import { jsonInit, request } from './library.ts';
import type { LibraryLineup } from './lineups.ts';
import type { PracticeSpot } from './practice.ts';
import type { TacticalGeometry, TacticalSide } from './tactical.ts';

/**
 * Browser-side contract of the pro lineup library (`prolineups.MapIndex` in Go):
 * grenades pros repeat, read from pro match demos on the ClipHub server.
 */

export const PRO_LINEUP_KINDS = ['smoke', 'flash', 'he', 'molotov'] as const;
export type ProLineupKind = (typeof PRO_LINEUP_KINDS)[number];

export type ProCounts = { recent: number; all: number };

export type ProUsage = { key: string; name: string; count: number; share: number; detail?: string };

export type ProExample = {
  match: string;
  map_key: string;
  event: string;
  played_at: string;
  team: string;
  player: string;
  round: number;
  /** Seconds after freeze end. */
  clock: number;
  tick: number;
  steamid64?: string;
  air_time?: number;
  /** Stored name of the recorded throw clip, when one exists. */
  clip?: string;
  /** For a combo example: every throw of the combo in clock order. */
  throws?: { lineup: string; player: string; steamid64: string; tick: number; air_time: number }[];
};

export type ProLineup = {
  id: string;
  slug: string;
  title: string;
  kind: ProLineupKind;
  side: TacticalSide;
  throw_place: string;
  land_place: string;
  /** Feet position, as `setpos` takes it. */
  origin: [number, number, number];
  /** Setang order: pitch from -90 (up) to 90 (down), yaw from -180 to 180. */
  yaw: number;
  pitch: number;
  landing: [number, number, number];
  movement: string;
  jump: boolean;
  insta: boolean;
  air_time: number;
  throws: ProCounts;
  matches: ProCounts;
  round_share: number;
  /** Throws per 100 side rounds: last 30 days, then the rest of the window. */
  trend?: [number, number];
  round_win: number;
  attempts: number;
  landed: number;
  clock: { median: number; buckets: number[] };
  teams: ProUsage[];
  players: ProUsage[];
  events: ProUsage[];
  plant_sites: ProUsage[];
  with: ProUsage[];
  examples: ProExample[];
  /** The newest recorded example's clip. */
  clip?: string;
  last_seen: string;
  team_count: number;
  rank: { position: number; of: number };
};

export type ProCombo = {
  id: string;
  title: string;
  side: TacticalSide;
  lineups: string[];
  rounds: ProCounts;
  team_count: number;
  round_win: number;
  clock: number;
  teams: ProUsage[];
  examples: ProExample[];
  /** The newest recorded example: its throws one after another. */
  clip?: string;
};

export type ProLineupMap = {
  version: number;
  map: string;
  built_at: string;
  recent_days: number;
  maps: ProCounts;
  rounds_t: ProCounts;
  rounds_ct: ProCounts;
  lineups: ProLineup[];
  combos: ProCombo[];
  latest_played: string;
  geometry?: TacticalGeometry;
  /** False when the server did not answer and this is the last copy Studio kept. */
  fresh: boolean;
};

export type ProLineupMapInfo = { map: string; lineups: number; combos: number; maps: number; recent_maps: number };

export type ProLineupMaps = { version: number; built_at: string; maps: ProLineupMapInfo[]; fresh: boolean };

export const PRO_LINEUP_MAPS_KEYS = ['version', 'built_at', 'maps', 'fresh'] as const satisfies readonly (keyof ProLineupMaps)[];
export const PRO_LINEUP_MAP_KEYS = [
  'version', 'map', 'built_at', 'recent_days', 'maps', 'rounds_t', 'rounds_ct', 'lineups', 'combos', 'latest_played', 'geometry', 'fresh',
] as const satisfies readonly (keyof ProLineupMap)[];

export function listProLineupMaps(): Promise<ProLineupMaps> {
  return request<ProLineupMaps>('/pro-lineups');
}

export function getProLineupMap(map: string): Promise<ProLineupMap> {
  return request<ProLineupMap>(`/pro-lineups/${encodeURIComponent(map)}`);
}

/** Copies a pro lineup into the Libreta and returns the saved lineup. */
export function saveProLineup(map: string, id: string): Promise<LibraryLineup> {
  return request<LibraryLineup>(`/pro-lineups/${encodeURIComponent(map)}/lineups/${encodeURIComponent(id)}/save`, jsonInit('POST'));
}

/** "Humo", "Flash"... for the four kinds a lineup can be. */
export const PRO_KIND_LABEL: Record<ProLineupKind, string> = { smoke: 'Humo', flash: 'Flash', he: 'HE', molotov: 'Molotov' };

/** How the throw is made, the way players say it. */
export function techniqueLabel(lineup: Pick<ProLineup, 'movement' | 'jump'>): string {
  const move: Record<string, string> = { running: 'corriendo', walking: 'andando', crouching: 'agachado', standing: 'quieto' };
  const base = move[lineup.movement] ?? 'quieto';
  if (!lineup.jump) return base === 'quieto' ? 'Lanzamiento normal' : `Lanzado ${base}`;
  if (base === 'corriendo') return 'Run jumpthrow';
  if (base === 'agachado') return 'Jumpthrow agachado';
  return 'Jumpthrow';
}

/** CS2 round clock for seconds after freeze end: 1:55 counting down. */
export function roundClock(seconds: number): string {
  const left = Math.max(0, Math.round(115 - seconds));
  return `${Math.floor(left / 60)}:${String(left % 60).padStart(2, '0')}`;
}

/** The console line that stands the player on the lineup. */
export function setposCommand(lineup: Pick<ProLineup, 'origin' | 'pitch' | 'yaw'>): string {
  const [x, y, z] = lineup.origin;
  return `setpos ${x.toFixed(2)} ${y.toFixed(2)} ${z.toFixed(2)};setang ${lineup.pitch.toFixed(2)} ${lineup.yaw.toFixed(2)} 0`;
}

export type ProLineupFilter = {
  kinds: ReadonlySet<ProLineupKind>;
  side: TacticalSide | '';
  insta: boolean;
  /** Only lineups with a recorded pro clip. */
  video: boolean;
  text: string;
};

/** The lineups a filter keeps, in the server's order (most thrown first). */
export function filterProLineups(lineups: readonly ProLineup[], filter: ProLineupFilter): ProLineup[] {
  const needle = filter.text.trim().toLowerCase();
  return lineups.filter(
    (l) =>
      filter.kinds.has(l.kind) &&
      (filter.side === '' || l.side === filter.side) &&
      (!filter.insta || l.insta) &&
      (!filter.video || Boolean(l.clip)) &&
      (needle === '' || l.title.toLowerCase().includes(needle) || l.land_place.toLowerCase().includes(needle) || l.throw_place.toLowerCase().includes(needle)),
  );
}

/** A share as "42 %". */
export function pct(share: number): string {
  return `${Math.round(share * 100)} %`;
}

export function lineupHref(map: string, lineup: Pick<ProLineup, 'id'>): string {
  return `/lineups/${map}/${encodeURIComponent(lineup.id)}`;
}

/** A lineup as a CS2 practice spot; a CT molotov is the incendiary the CT buys. */
export function practiceSpot(lineup: ProLineup): PracticeSpot {
  return {
    label: lineup.title,
    pos: lineup.origin,
    ang: [lineup.pitch, lineup.yaw],
    grenade: lineup.kind === 'molotov' && lineup.side === 'CT' ? 'incendiary' : lineup.kind,
  };
}

/** The Studio URL of a recorded throw clip; the orchestrator keeps it after the first view. */
export function proClipUrl(clip: string): string {
  return `/api/library/pro-lineups/clips/${clip.split('/').map(encodeURIComponent).join('/')}`;
}

/** The clip to play: the chosen recorded example, else the lineup's own clip; null when none is recorded. */
export function shownClip(
  lineup: Pick<ProLineup, 'clip' | 'examples'>,
  chosen: string | null,
): { clip: string; example: ProExample } | null {
  const find = (want: string | null | undefined) => lineup.examples.find((e) => e.clip !== undefined && e.clip === want);
  // A choice from another lineup falls back to this lineup's own clip.
  const example = find(chosen) ?? find(lineup.clip);
  return example?.clip === undefined ? null : { clip: example.clip, example };
}
