import { jsonInit, request } from './library.ts';
import type { RadarLevel, TacticalGeometry } from './tactical.ts';

/**
 * Browser-side contract of the team's own callout names (`library.CalloutName`
 * and GET /maps/{map}/callouts): a name per CS2 place of a map.
 */

/** The server's cap on a name (`library.MaxCalloutNameRunes`). */
export const MAX_CALLOUT_NAME = 40;

/** A CS2 place of the map, with the team's name for it when it has one. */
export type MapCallout = {
  token: string;
  name?: string;
  level?: RadarLevel;
  /** World x, y; missing for a named place the reference geometry lacks. */
  center?: [number, number];
};

export type MapCallouts = {
  map: string;
  /** The analysed match the places come from; missing when the map has none. */
  reference_match_id?: string;
  /** Only with `geometry: true` and an analysed match. */
  geometry?: TacticalGeometry;
  callouts: MapCallout[];
};

export type CalloutName = { map: string; token: string; name: string; updated_at?: string };

export const MAP_CALLOUTS_KEYS = ['map', 'reference_match_id', 'geometry', 'callouts'] as const satisfies readonly (keyof MapCallouts)[];
export const MAP_CALLOUTS_QUERY = ['geometry'] as const;
export const CALLOUT_NAME_KEYS = ['map', 'token', 'name', 'updated_at'] as const satisfies readonly (keyof CalloutName)[];
export const CALLOUT_NAME_FIELDS = ['name'] as const;

function calloutsPath(map: string): string {
  return `/maps/${encodeURIComponent(map)}/callouts`;
}

/** The places of a map and the team's names for them; `geometry` also returns the map to draw. */
export function getMapCallouts(map: string, options: { geometry?: boolean } = {}): Promise<MapCallouts> {
  return request<MapCallouts>(`${calloutsPath(map)}${options.geometry ? '?geometry=true' : ''}`);
}

export function setCalloutName(name: { map: string; token: string; name: string }): Promise<CalloutName> {
  return request<CalloutName>(`${calloutsPath(name.map)}/${encodeURIComponent(name.token)}`, jsonInit('PUT', { name: name.name }));
}

/** Back to CS2's name; a 404 means the place had no name of the team. */
export function deleteCalloutName(map: string, token: string): Promise<void> {
  return request<void>(`${calloutsPath(map)}/${encodeURIComponent(token)}`, { method: 'DELETE' });
}
