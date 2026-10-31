// Node-resolvable specifiers so the colocated node:test suite imports this
// module directly, like app/api/demos/_lib.ts.
import { TACTICAL_DOCUMENT_KEYS, TACTICAL_FILTER_PARAM_NAMES, TACTICAL_ROUND_KEYS, TACTICAL_TENDENCIES_KEYS } from '../../../lib/api/tactical.ts';
import {
  ANNOTATION_KEYS,
  HEATMAP_QUERY,
  OVERLAY_QUERY,
  SPATIAL_RESPONSE_KEYS,
  TEAM_SPATIAL_QUERY,
} from '../../../lib/api/spatial.ts';
import { METRICS_KEYS, PLAYER_METRICS_QUERY } from '../../../lib/api/metrics.ts';
import {
  PLAY_CREATE_FIELDS,
  PLAY_DETAIL_KEYS,
  PLAY_DETECTION_KEYS,
  PLAY_KEYS,
  PLAY_LIST_QUERY,
  PLAY_PATCH_FIELDS,
  ROUND_LINEUPS_KEYS,
  STRATBOOK_FILE_KEYS,
  STRATBOOK_IMPORT_KEYS,
} from '../../../lib/api/stratbook.ts';
import { SERIES_FIELDS, SERIES_KEYS, TEAM_MAPS_KEYS, VETO_KEYS, VETO_QUERY } from '../../../lib/api/veto.ts';

/**
 * The `/api/library/*` routes the browser may reach, as a whitelist. A request
 * that matches no entry never reaches the orchestrator; a matching one carries
 * only the query names, body fields and response keys listed for it, so a new
 * orchestrator field cannot leak through the proxy.
 */

export type LibraryMethod = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

export type LibraryRoute = {
  readonly method: LibraryMethod;
  /** Path segments after `/api/library/`; a RegExp segment is a validated parameter. */
  readonly segments: readonly (string | RegExp)[];
  readonly query?: readonly string[];
  /**
   * `json` forwards an object body with these fields only, up to `maxBytes`
   * (the 1 MiB control limit when unset); `multipart` streams the import.
   */
  readonly body?: { readonly kind: 'json'; readonly fields: readonly string[]; readonly maxBytes?: number } | { readonly kind: 'multipart' };
  /** `json` copies these top-level keys; `stream` proxies bytes; `empty` is a 204. */
  readonly response: { readonly kind: 'json'; readonly keys: readonly string[] } | { readonly kind: 'stream' } | { readonly kind: 'empty' };
};

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const STEAM_ID = /^[0-9]{1,20}$/;
const TEAM_KEY = /^(ct-start|t-start)$/;
const ROUND = /^[1-9][0-9]{0,2}$/;

const PAGING = ['limit', 'offset'];
/** A stratbook file holds up to 1000 plays, so its import gets the 8 MiB the orchestrator reads. */
const STRATBOOK_FILE_BYTES = 8 * 1024 * 1024;

export const LIBRARY_DEMO_KEYS = [
  'id', 'sha256', 'file_name', 'path', 'size_bytes', 'source', 'source_manual', 'cs2_build', 'patch_version',
  'state', 'error_code', 'error', 'tags', 'job_ids', 'match_id', 'played_at', 'imported_at', 'updated_at',
];
const MATCH_KEYS = [
  'id', 'demo_id', 'series_id', 'map', 'played_at', 'source', 'tickrate', 'duration_ticks', 'round_count',
  'teams', 'players', 'created_at', 'updated_at',
];
const TEAM_KEYS = ['id', 'name', 'aliases', 'match_count', 'roster', 'created_at', 'updated_at'];
const PLAYER_KEYS = [
  'steamid64', 'nick', 'nicks', 'faceit_player_id', 'faceit_nickname', 'match_count', 'teams', 'created_at', 'updated_at',
];
const ROLE_KEYS = [
  'steamid64', 'role', 'match_count', 'rounds', 'awp_kills', 't_openings', 'utility', 'team_utility', 'thresholds',
];

const REPORT_KEYS = ['id', 'title', 'team_id', 'team_name', 'match_ids', 'side', 'notes', 'created_at', 'updated_at'];
const SCOUT_KEYS = [
  'team_id', 'team_name', 'skipped', 'filter', 'thresholds', 'matches', 'maps', 'tendencies', 'pistols',
  't_defaults', 'ct_setups', 'executes', 'economy', 'awp', 'openings',
];
const REPORT_FIELDS = ['title', 'match_ids', 'side', 'notes'];
/** A note moves along its round but never to another round or match. */
const ANNOTATION_FIELDS = ['tick', 'text', 'strokes'];

const json = (keys: readonly string[]) => ({ kind: 'json', keys }) as const;
const fields = (names: readonly string[]) => ({ kind: 'json', fields: names }) as const;

export const LIBRARY_ROUTES: readonly LibraryRoute[] = [
  { method: 'GET', segments: ['demos'], query: ['state', 'source', 'tag', ...PAGING], response: json(['demos']) },
  { method: 'POST', segments: ['demos'], body: { kind: 'multipart' }, response: json(['demos']) },
  { method: 'GET', segments: ['demos', UUID], response: json(LIBRARY_DEMO_KEYS) },
  { method: 'PATCH', segments: ['demos', UUID], body: fields(['source', 'tags']), response: json(LIBRARY_DEMO_KEYS) },
  { method: 'DELETE', segments: ['demos', UUID], response: { kind: 'empty' } },
  { method: 'POST', segments: ['demos', UUID, 'analyze'], response: json(LIBRARY_DEMO_KEYS) },
  { method: 'POST', segments: ['demos', UUID, 'jobs'], body: fields(['target_steamid']), response: json(['job_id', 'status', 'reused']) },
  { method: 'GET', segments: ['matches'], query: ['map', 'team_id', 'steamid', 'series_id', 'source', 'tag', 'from', 'to', ...PAGING], response: json(['matches']) },
  { method: 'GET', segments: ['matches', UUID], response: json(MATCH_KEYS) },
  { method: 'PATCH', segments: ['matches', UUID], body: fields(['played_at', 'series_id']), response: json(MATCH_KEYS) },
  { method: 'PUT', segments: ['matches', UUID, 'teams', TEAM_KEY], body: fields(['team_id', 'team_name', 'remember_alias']), response: json(MATCH_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical'], response: json(TACTICAL_DOCUMENT_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'rounds', ROUND], response: json(TACTICAL_ROUND_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'positions'], response: { kind: 'stream' } },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'aggregate'], query: TACTICAL_FILTER_PARAM_NAMES, response: json(TACTICAL_TENDENCIES_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'heatmap'], query: [...TACTICAL_FILTER_PARAM_NAMES, ...HEATMAP_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'overlay'], query: [...TACTICAL_FILTER_PARAM_NAMES, ...OVERLAY_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'rounds', ROUND, 'play'], query: ['team_key', 'tick'], response: json(['draft']) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'rounds', ROUND, 'lineups'], response: json(ROUND_LINEUPS_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'metrics'], response: json(METRICS_KEYS) },
  { method: 'GET', segments: ['teams'], query: ['q', ...PAGING], response: json(['teams']) },
  { method: 'GET', segments: ['teams', UUID], response: json(TEAM_KEYS) },
  { method: 'PATCH', segments: ['teams', UUID], body: fields(['name', 'aliases']), response: json(TEAM_KEYS) },
  { method: 'POST', segments: ['teams', UUID, 'merge'], body: fields(['into_team_id']), response: json(TEAM_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'scout'], query: ['map', 'side', 'from', 'to', 'opponent_id', 'match_id'], response: json(SCOUT_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'heatmap'], query: [...TEAM_SPATIAL_QUERY, ...HEATMAP_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'overlay'], query: [...TEAM_SPATIAL_QUERY, ...OVERLAY_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'plays'], query: ['map', 'side', 'from', 'to', 'opponent_id', 'match_id'], response: json(PLAY_DETECTION_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'maps'], query: ['from', 'to'], response: json(TEAM_MAPS_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'veto'], query: VETO_QUERY, response: json(VETO_KEYS) },
  { method: 'GET', segments: ['series'], query: ['team_id', ...PAGING], response: json(['series']) },
  { method: 'POST', segments: ['series'], body: fields(SERIES_FIELDS), response: json(SERIES_KEYS) },
  { method: 'GET', segments: ['series', UUID], response: json(SERIES_KEYS) },
  { method: 'PATCH', segments: ['series', UUID], body: fields(SERIES_FIELDS), response: json(SERIES_KEYS) },
  { method: 'DELETE', segments: ['series', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['plays'], query: [...PLAY_LIST_QUERY, ...PAGING], response: json(['plays']) },
  { method: 'POST', segments: ['plays'], body: fields(PLAY_CREATE_FIELDS), response: json(PLAY_KEYS) },
  { method: 'GET', segments: ['plays', 'export'], query: PLAY_LIST_QUERY, response: json(STRATBOOK_FILE_KEYS) },
  {
    method: 'POST',
    segments: ['plays', 'import'],
    body: { kind: 'json', fields: STRATBOOK_FILE_KEYS, maxBytes: STRATBOOK_FILE_BYTES },
    response: json(STRATBOOK_IMPORT_KEYS),
  },
  { method: 'GET', segments: ['plays', UUID], response: json(PLAY_DETAIL_KEYS) },
  { method: 'PATCH', segments: ['plays', UUID], body: fields(PLAY_PATCH_FIELDS), response: json(PLAY_KEYS) },
  { method: 'DELETE', segments: ['plays', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['reports'], query: ['team_id', ...PAGING], response: json(['reports']) },
  { method: 'POST', segments: ['reports'], body: fields(['team_id', ...REPORT_FIELDS]), response: json(REPORT_KEYS) },
  { method: 'GET', segments: ['reports', UUID], response: json(REPORT_KEYS) },
  { method: 'PATCH', segments: ['reports', UUID], body: fields(REPORT_FIELDS), response: json(REPORT_KEYS) },
  { method: 'DELETE', segments: ['reports', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['reports', UUID, 'scout'], response: json(SCOUT_KEYS) },
  { method: 'GET', segments: ['reports', UUID, 'metrics'], response: json(METRICS_KEYS) },
  { method: 'GET', segments: ['annotations'], query: ['match_id', 'round'], response: json(['annotations']) },
  { method: 'POST', segments: ['annotations'], body: fields(['match_id', 'round', ...ANNOTATION_FIELDS]), response: json(ANNOTATION_KEYS) },
  { method: 'PATCH', segments: ['annotations', UUID], body: fields(ANNOTATION_FIELDS), response: json(ANNOTATION_KEYS) },
  { method: 'DELETE', segments: ['annotations', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['players'], query: ['q', 'team_id', ...PAGING], response: json(['players']) },
  { method: 'GET', segments: ['players', STEAM_ID], response: json(PLAYER_KEYS) },
  { method: 'POST', segments: ['players', STEAM_ID, 'faceit'], response: json(PLAYER_KEYS) },
  { method: 'GET', segments: ['players', STEAM_ID, 'role'], response: json(ROLE_KEYS) },
  { method: 'GET', segments: ['players', STEAM_ID, 'metrics'], query: PLAYER_METRICS_QUERY, response: json(METRICS_KEYS) },
  { method: 'GET', segments: ['jobs'], response: json(['unlinked_job_ids']) },
  { method: 'POST', segments: ['jobs'], response: json(['demos']) },
  { method: 'GET', segments: ['jobs', UUID], response: json(LIBRARY_DEMO_KEYS) },
  { method: 'POST', segments: ['jobs', UUID], response: json(LIBRARY_DEMO_KEYS) },
];

/** The route a request addresses, or undefined when the whitelist has none. */
export function matchLibraryRoute(method: string, path: readonly string[]): LibraryRoute | undefined {
  return LIBRARY_ROUTES.find(
    (route) =>
      route.method === method.toUpperCase()
      && route.segments.length === path.length
      && route.segments.every((segment, i) => {
        const part = path[i] ?? '';
        return typeof segment === 'string' ? segment === part : segment.test(part);
      }),
  );
}

/** The upstream query: only the route's names, empty values dropped, repeats kept in order. */
export function libraryUpstreamQuery(route: LibraryRoute, search: URLSearchParams): string {
  const out = new URLSearchParams();
  for (const name of route.query ?? []) {
    for (const value of search.getAll(name)) {
      if (value !== '') out.append(name, value);
    }
  }
  return out.toString();
}

/** Copies only the listed top-level keys of an upstream object. */
export function pickLibraryKeys(body: unknown, keys: readonly string[]): Record<string, unknown> {
  const out: Record<string, unknown> = {};
  if (body === null || typeof body !== 'object' || Array.isArray(body)) return out;
  for (const key of keys) {
    const value: unknown = Reflect.get(body, key);
    if (value !== undefined) out[key] = value;
  }
  return out;
}
