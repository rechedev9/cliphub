// Node-resolvable specifiers so the colocated node:test suite imports this
// module directly, like app/api/demos/_lib.ts.
import { TACTICAL_DOCUMENT_KEYS, TACTICAL_FILTER_PARAM_NAMES, TACTICAL_ROUND_KEYS, TACTICAL_TENDENCIES_KEYS } from '../../../lib/api/tactical.ts';
import {
  ANNOTATION_KEYS,
  FOCUS_QUERY,
  HEATMAP_QUERY,
  OVERLAY_QUERY,
  SPATIAL_RESPONSE_KEYS,
  TEAM_SPATIAL_QUERY,
} from '../../../lib/api/spatial.ts';
import { METRICS_KEYS, PLAYER_METRICS_QUERY } from '../../../lib/api/metrics.ts';
import { BASELINE_KEYS, BASELINE_QUERY, LEADERBOARD_KEYS, LEADERBOARD_QUERY } from '../../../lib/api/baseline.ts';
import { NORMS_KEYS, NORMS_QUERY } from '../../../lib/api/norms.ts';
import { PINNED_PLAYERS_FIELDS, PINNED_PLAYERS_KEYS } from '../../../lib/api/pinned-players.ts';
import { PLAYBOOK_KEYS, PLAYBOOK_QUERY, PREP_KEYS, PREP_QUERY } from '../../../lib/api/playbook.ts';
import { ATTACK_KEYS, ATTACK_QUERY } from '../../../lib/api/attack.ts';
import { DATA_WARNINGS_KEYS, DATA_WARNINGS_QUERY, FIRST_DEATH_KEYS, FIRST_DEATH_QUERY } from '../../../lib/api/first-death.ts';
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
import { TEAM_GRENADES_KEYS, TEAM_GRENADES_QUERY } from '../../../lib/api/grenades.ts';
import { LINEUP_FIELDS, LINEUP_KEYS, LINEUP_LIST_QUERY, LINEUP_PATCH_FIELDS } from '../../../lib/api/lineups.ts';
import { PRO_LINEUP_MAP_KEYS, PRO_LINEUP_MAPS_KEYS } from '../../../lib/api/pro-lineups.ts';
import { BOT_REPLAY_FIELDS, BOT_REPLAY_KEYS } from '../../../lib/api/bot-replay.ts';
import { PRO_FOLLOW_KEYS, PRO_IMPORT_KEYS, PRO_MATCH_INFO_KEYS, PRO_MATCH_QUERY, PRO_MATCHES_KEYS } from '../../../lib/api/pro-matches.ts';
import { ROLE_SHEET_FIELDS, ROLE_SHEET_KEYS, TEAM_ROLES_KEYS } from '../../../lib/api/roles.ts';
import { CALL_FIELDS, CALL_KEYS, CALL_LIST_QUERY } from '../../../lib/api/calls.ts';
import {
  MATCH_PATTERNS_KEYS,
  PATTERN_FIELDS,
  PATTERN_LIST_QUERY,
  PATTERN_RUN_KEYS,
  SAVED_PATTERN_KEYS,
  TEAM_PATTERN_QUERY,
} from '../../../lib/api/patterns.ts';
import {
  PLAYLIST_ADD_FIELDS,
  PLAYLIST_FIELDS,
  PLAYLIST_FILE_KEYS,
  PLAYLIST_IMPORT_KEYS,
  PLAYLIST_KEYS,
  PLAYLIST_LIST_QUERY,
  PLAYLIST_ROUND_FIELDS,
} from '../../../lib/api/playlists.ts';
import {
  CALLOUT_NAME_FIELDS,
  CALLOUT_NAME_KEYS,
  MAP_CALLOUTS_KEYS,
  MAP_CALLOUTS_QUERY,
} from '../../../lib/api/callouts.ts';
import { SHARE_IMPORT_KEYS } from '../../../lib/library/share.ts';
import { SEARCH_QUERY_KEYS, SEARCH_RESPONSE_KEYS } from '../../../lib/api/search.ts';
import { SERIES_FIELDS, SERIES_KEYS, TEAM_MAP_COUNTS_KEYS, TEAM_MAPS_KEYS, VETO_KEYS, VETO_QUERY } from '../../../lib/api/veto.ts';
import {
  DEMO_VOICE_KEYS,
  TRANSCRIBER_KEYS,
  VOICE_COMMS_KEYS,
  VOICE_COMMS_QUERY,
  VOICE_IMPORT_KEYS,
  VOICE_KEYS,
  VOICE_SEARCH_KEYS,
  VOICE_SEARCH_QUERY,
  VOICE_SUMMARY_KEYS,
  VOICE_SYNC_FIELDS,
  VOICE_TRACK_FIELDS,
  VOICE_TRANSCRIBE_FIELDS,
  VOICE_UPLOAD_BYTES,
} from '../../../lib/api/voice.ts';
import { REPLAY_VIDEO_BYTES, REPLAY_VIDEO_KEYS } from '../../../lib/api/replay-video.ts';
import {
  RECORDING_ATTACH_FIELDS,
  RECORDING_KEYS,
  RECORDING_LIST_KEYS,
  VOICE_PLUGIN_KEYS,
  VOICE_SPEAKERS_FIELDS,
} from '../../../lib/api/voice-plugin.ts';
import {
  PLAYER_POSITIONS_KEYS,
  PLAYER_POSITIONS_QUERY,
  TEAM_BOMBSITES_KEYS,
  TEAM_BOMBSITES_QUERY,
  TEAM_LINEUPS_KEYS,
  TEAM_PROFILE_FIELDS,
  TEAM_PROFILE_KEYS,
  TEAM_ROUNDS_KEYS,
  TEAM_SCOPE_QUERY,
} from '../../../lib/api/team-insights.ts';
import { TEAM_SIDE_STATS_KEYS, TEAM_SIDE_STATS_QUERY } from '../../../lib/api/side-stats.ts';

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
   * (the 1 MiB control limit when unset); `multipart` streams the import, up to `maxBytes` (one demo when unset).
   */
  readonly body?:
    | { readonly kind: 'json'; readonly fields: readonly string[]; readonly maxBytes?: number }
    | { readonly kind: 'multipart'; readonly maxBytes?: number };
  /**
   * `json` copies these top-level keys; `stream` proxies bytes; `empty` is a 204;
   * `download` returns the bytes of a POST as a file with its name and length.
   */
  readonly response:
    | { readonly kind: 'json'; readonly keys: readonly string[] }
    | { readonly kind: 'stream' }
    | { readonly kind: 'empty' }
    | { readonly kind: 'download' };
};

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;
const STEAM_ID = /^[0-9]{1,20}$/;
const TEAM_KEY = /^(ct-start|t-start)$/;
const ROUND = /^[1-9][0-9]{0,2}$/;
const MAP = /^[a-z0-9_]{1,64}$/;
/** A pro lineup id: kind, side, then the rounded throw and landing cells and the jump flag. */
const PRO_LINEUP_ID = /^(smoke|flash|he|molotov)-(t|ct)(-(-?[0-9]{1,4})){4}-[01](-[0-9]{1,3})?$/;
/** A recorded pro throw clip: its match folder, then map, round, tick and thrower. */
const PRO_CLIP_MATCH = /^[0-9]{1,12}$/;
const PRO_CLIP_FILE = /^[a-z0-9-]{1,220}-r[0-9]{1,3}-t[0-9]{1,9}-[0-9]{17}(-combo)?\.mp4$/;
/** A CS2 place name (`m_szLastPlaceName`) such as `BombsiteA` or `TopofMid`. */
/** An HLTV match or team id, and a pro map's demo entry without .dem. */
const HLTV_ID = /^[1-9][0-9]{0,11}$/;
const PRO_ENTRY = /^[A-Za-z0-9][A-Za-z0-9_.()-]{0,200}$/;
const PLACE = /^[A-Za-z0-9_][A-Za-z0-9_ '.-]{0,63}$/;
/** A team voice track number, 1 to 16. */
const VOICE_TRACK = /^([1-9]|1[0-6])$/;
/** A TeamSpeak plugin recording: its UTC start and 4 hex digits. */
const RECORDING_ID = /^[0-9]{8}-[0-9]{6}-[0-9a-f]{4}$/;

const PAGING = ['limit', 'offset'];
/** A Libreta file holds up to 1000 plays and its other sections, so its import gets the 8 MiB the orchestrator reads. */
const STRATBOOK_FILE_BYTES = 8 * 1024 * 1024;

export const LIBRARY_DEMO_KEYS = [
  'id', 'sha256', 'file_name', 'path', 'size_bytes', 'source', 'source_manual', 'cs2_build', 'patch_version',
  'state', 'error_code', 'error', 'tags', 'job_ids', 'match_id', 'played_at', 'imported_at', 'updated_at',
];
const MATCH_KEYS = [
  'id', 'demo_id', 'series_id', 'map', 'played_at', 'source', 'tickrate', 'duration_ticks', 'round_count',
  'teams', 'players', 'created_at', 'updated_at',
];
const SCOUT_QUERY = ['map', 'side', 'from', 'to', 'opponent_id', 'match_id', 'lineup'];
const TEAM_KEYS = ['id', 'name', 'aliases', 'match_count', 'roster', 'own', 'hltv_rank', 'created_at', 'updated_at'];
const PLAYER_KEYS = [
  'steamid64', 'nick', 'nicks', 'faceit_player_id', 'faceit_nickname', 'match_count', 'teams', 'created_at', 'updated_at',
];
const ROLE_KEYS = [
  'steamid64', 'role', 'match_count', 'rounds', 'awp_kills', 't_openings', 'utility', 'team_utility', 'thresholds',
];

const REPORT_KEYS = ['id', 'title', 'team_id', 'team_name', 'match_ids', 'side', 'notes', 'round_lists', 'created_at', 'updated_at'];
const SCOUT_KEYS = [
  'team_id', 'team_name', 'skipped', 'filter', 'thresholds', 'matches', 'maps', 'tendencies', 'pistols',
  't_defaults', 'ct_setups', 'executes', 'economy', 'awp', 'openings',
];
const REPORT_FIELDS = ['title', 'match_ids', 'side', 'notes', 'round_lists'];
/** The sender's optional message, written into the package manifest. */
const SHARE_EXPORT_FIELDS = ['notes'];
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
  { method: 'GET', segments: ['matches', UUID, 'pro'], response: json(PRO_MATCH_INFO_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'patterns'], query: ['round'], response: json(MATCH_PATTERNS_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical'], response: json(TACTICAL_DOCUMENT_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'rounds', ROUND], response: json(TACTICAL_ROUND_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'positions'], response: { kind: 'stream' } },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'aggregate'], query: TACTICAL_FILTER_PARAM_NAMES, response: json(TACTICAL_TENDENCIES_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'heatmap'], query: [...TACTICAL_FILTER_PARAM_NAMES, ...FOCUS_QUERY, ...HEATMAP_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'overlay'], query: [...TACTICAL_FILTER_PARAM_NAMES, ...FOCUS_QUERY, ...OVERLAY_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'rounds', ROUND, 'play'], query: ['team_key', 'tick'], response: json(['draft']) },
  { method: 'GET', segments: ['matches', UUID, 'tactical', 'rounds', ROUND, 'lineups'], response: json(ROUND_LINEUPS_KEYS) },
  {
    method: 'POST',
    segments: ['matches', UUID, 'tactical', 'rounds', ROUND, 'bot-replay'],
    body: fields(BOT_REPLAY_FIELDS),
    response: json(BOT_REPLAY_KEYS),
  },
  { method: 'GET', segments: ['matches', UUID, 'metrics'], response: json(METRICS_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'voice'], response: json(VOICE_KEYS) },
  { method: 'POST', segments: ['matches', UUID, 'voice'], body: { kind: 'multipart', maxBytes: VOICE_UPLOAD_BYTES }, response: json(VOICE_IMPORT_KEYS) },
  { method: 'PATCH', segments: ['matches', UUID, 'voice'], body: fields(VOICE_SYNC_FIELDS), response: json(VOICE_KEYS) },
  { method: 'DELETE', segments: ['matches', UUID, 'voice'], response: { kind: 'empty' } },
  { method: 'PATCH', segments: ['matches', UUID, 'voice', 'tracks', VOICE_TRACK], body: fields(VOICE_TRACK_FIELDS), response: json(VOICE_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'voice', 'tracks', VOICE_TRACK, 'audio'], response: { kind: 'stream' } },
  { method: 'POST', segments: ['matches', UUID, 'voice', 'transcribe'], body: fields(VOICE_TRANSCRIBE_FIELDS), response: json(VOICE_KEYS) },
  { method: 'POST', segments: ['matches', UUID, 'replay-video'], body: { kind: 'multipart', maxBytes: REPLAY_VIDEO_BYTES }, response: json(REPLAY_VIDEO_KEYS) },
  { method: 'GET', segments: ['replay-videos', UUID], response: { kind: 'stream' } },
  { method: 'GET', segments: ['matches', UUID, 'voice', 'comms'], query: VOICE_COMMS_QUERY, response: json(VOICE_COMMS_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'voice', 'demo'], response: json(DEMO_VOICE_KEYS) },
  { method: 'POST', segments: ['matches', UUID, 'voice', 'demo'], response: json(DEMO_VOICE_KEYS) },
  { method: 'GET', segments: ['matches', UUID, 'voice', 'demo', STEAM_ID, 'audio'], response: { kind: 'stream' } },
  { method: 'GET', segments: ['voice', 'summary'], response: json(VOICE_SUMMARY_KEYS) },
  { method: 'GET', segments: ['voice', 'search'], query: VOICE_SEARCH_QUERY, response: json(VOICE_SEARCH_KEYS) },
  { method: 'GET', segments: ['voice', 'transcriber'], response: json(TRANSCRIBER_KEYS) },
  { method: 'POST', segments: ['voice', 'transcriber', 'install'], response: json(TRANSCRIBER_KEYS) },
  { method: 'PUT', segments: ['matches', UUID, 'voice', 'speakers'], body: fields(VOICE_SPEAKERS_FIELDS), response: json(VOICE_KEYS) },
  { method: 'GET', segments: ['voice', 'recordings'], response: json(RECORDING_LIST_KEYS) },
  { method: 'POST', segments: ['voice', 'recordings', RECORDING_ID, 'attach'], body: fields(RECORDING_ATTACH_FIELDS), response: json(RECORDING_KEYS) },
  { method: 'DELETE', segments: ['voice', 'recordings', RECORDING_ID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['voice', 'plugin'], response: json(VOICE_PLUGIN_KEYS) },
  { method: 'POST', segments: ['voice', 'plugin', 'setup'], response: json(VOICE_PLUGIN_KEYS) },
  { method: 'GET', segments: ['teams'], query: ['q', ...PAGING], response: json(['teams']) },
  { method: 'GET', segments: ['teams', UUID], response: json(TEAM_KEYS) },
  { method: 'PATCH', segments: ['teams', UUID], body: fields(['name', 'aliases']), response: json(TEAM_KEYS) },
  { method: 'POST', segments: ['teams', UUID, 'merge'], body: fields(['into_team_id']), response: json(TEAM_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'lineups'], response: json(TEAM_LINEUPS_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'profile'], response: json(TEAM_PROFILE_KEYS) },
  { method: 'PUT', segments: ['teams', UUID, 'profile'], body: fields(TEAM_PROFILE_FIELDS), response: json(TEAM_PROFILE_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'rounds'], query: [...SCOUT_QUERY, ...TEAM_SCOPE_QUERY], response: json(TEAM_ROUNDS_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'scout'], query: [...SCOUT_QUERY, ...TEAM_SCOPE_QUERY], response: json(SCOUT_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'heatmap'], query: [...TEAM_SPATIAL_QUERY, ...TEAM_SCOPE_QUERY, ...HEATMAP_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'overlay'], query: [...TEAM_SPATIAL_QUERY, ...TEAM_SCOPE_QUERY, ...OVERLAY_QUERY], response: json(SPATIAL_RESPONSE_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'plays'], query: [...SCOUT_QUERY, ...TEAM_SCOPE_QUERY], response: json(PLAY_DETECTION_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'grenades'], query: [...TEAM_GRENADES_QUERY, ...TEAM_SCOPE_QUERY], response: json(TEAM_GRENADES_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'playbook'], query: [...PLAYBOOK_QUERY, ...TEAM_SCOPE_QUERY], response: json(PLAYBOOK_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'prep'], query: [...PREP_QUERY, ...TEAM_SCOPE_QUERY], response: json(PREP_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'attack'], query: ATTACK_QUERY, response: json(ATTACK_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'first-death'], query: [...FIRST_DEATH_QUERY, ...TEAM_SCOPE_QUERY], response: json(FIRST_DEATH_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'warnings'], query: [...DATA_WARNINGS_QUERY, ...TEAM_SCOPE_QUERY], response: json(DATA_WARNINGS_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'roles'], query: PREP_QUERY, response: json(TEAM_ROLES_KEYS) },
  { method: 'PUT', segments: ['teams', UUID, 'roles'], body: fields(ROLE_SHEET_FIELDS), response: json(ROLE_SHEET_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'maps'], query: ['from', 'to'], response: json(TEAM_MAPS_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'map-counts'], query: [...SCOUT_QUERY, ...TEAM_SCOPE_QUERY], response: json(TEAM_MAP_COUNTS_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'veto'], query: VETO_QUERY, response: json(VETO_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'bombsites'], query: [...TEAM_BOMBSITES_QUERY, ...TEAM_SCOPE_QUERY], response: json(TEAM_BOMBSITES_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'side-stats'], query: [...TEAM_SIDE_STATS_QUERY, ...TEAM_SCOPE_QUERY], response: json(TEAM_SIDE_STATS_KEYS) },
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
  { method: 'GET', segments: ['lineups'], query: [...LINEUP_LIST_QUERY, ...PAGING], response: json(['lineups']) },
  { method: 'POST', segments: ['lineups'], body: fields(LINEUP_FIELDS), response: json(LINEUP_KEYS) },
  { method: 'GET', segments: ['lineups', UUID], response: json(LINEUP_KEYS) },
  { method: 'PATCH', segments: ['lineups', UUID], body: fields(LINEUP_PATCH_FIELDS), response: json(LINEUP_KEYS) },
  { method: 'DELETE', segments: ['lineups', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['calls'], query: [...CALL_LIST_QUERY, ...PAGING], response: json(['calls']) },
  { method: 'POST', segments: ['calls'], body: fields(CALL_FIELDS), response: json(CALL_KEYS) },
  { method: 'GET', segments: ['calls', UUID], response: json(CALL_KEYS) },
  { method: 'PATCH', segments: ['calls', UUID], body: fields(CALL_FIELDS), response: json(CALL_KEYS) },
  { method: 'DELETE', segments: ['calls', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['playlists'], query: [...PLAYLIST_LIST_QUERY, ...PAGING], response: json(['playlists']) },
  { method: 'POST', segments: ['playlists'], body: fields(PLAYLIST_FIELDS), response: json(PLAYLIST_KEYS) },
  { method: 'GET', segments: ['playlists', 'export'], query: ['id', ...PLAYLIST_LIST_QUERY], response: json(PLAYLIST_FILE_KEYS) },
  {
    method: 'POST',
    segments: ['playlists', 'import'],
    body: { kind: 'json', fields: PLAYLIST_FILE_KEYS, maxBytes: STRATBOOK_FILE_BYTES },
    response: json(PLAYLIST_IMPORT_KEYS),
  },
  { method: 'GET', segments: ['playlists', UUID], response: json(PLAYLIST_KEYS) },
  { method: 'PATCH', segments: ['playlists', UUID], body: fields(PLAYLIST_FIELDS), response: json(PLAYLIST_KEYS) },
  { method: 'DELETE', segments: ['playlists', UUID], response: { kind: 'empty' } },
  { method: 'POST', segments: ['playlists', UUID, 'rounds'], body: fields(PLAYLIST_ADD_FIELDS), response: json(PLAYLIST_KEYS) },
  {
    method: 'PATCH',
    segments: ['playlists', UUID, 'rounds', UUID, ROUND],
    // A round's drawings can pass the default body cap.
    body: { kind: 'json', fields: PLAYLIST_ROUND_FIELDS, maxBytes: STRATBOOK_FILE_BYTES },
    response: json(PLAYLIST_KEYS),
  },
  { method: 'DELETE', segments: ['playlists', UUID, 'rounds', UUID, ROUND], response: json(PLAYLIST_KEYS) },
  { method: 'GET', segments: ['teams', UUID, 'pattern'], query: TEAM_PATTERN_QUERY, response: json(PATTERN_RUN_KEYS) },
  { method: 'GET', segments: ['patterns'], query: PATTERN_LIST_QUERY, response: json(['patterns']) },
  { method: 'POST', segments: ['patterns'], body: fields(PATTERN_FIELDS), response: json(SAVED_PATTERN_KEYS) },
  { method: 'GET', segments: ['patterns', UUID], response: json(SAVED_PATTERN_KEYS) },
  { method: 'PATCH', segments: ['patterns', UUID], body: fields(PATTERN_FIELDS), response: json(SAVED_PATTERN_KEYS) },
  { method: 'DELETE', segments: ['patterns', UUID], response: { kind: 'empty' } },
  { method: 'GET', segments: ['patterns', UUID, 'rounds'], query: [...TEAM_PATTERN_QUERY, 'team_id'], response: json(PATTERN_RUN_KEYS) },
  { method: 'GET', segments: ['maps', MAP, 'callouts'], query: MAP_CALLOUTS_QUERY, response: json(MAP_CALLOUTS_KEYS) },
  { method: 'PUT', segments: ['maps', MAP, 'callouts', PLACE], body: fields(CALLOUT_NAME_FIELDS), response: json(CALLOUT_NAME_KEYS) },
  { method: 'DELETE', segments: ['maps', MAP, 'callouts', PLACE], response: { kind: 'empty' } },
  { method: 'GET', segments: ['libreta', 'export'], response: json(STRATBOOK_FILE_KEYS) },
  { method: 'GET', segments: ['search'], query: SEARCH_QUERY_KEYS, response: json(SEARCH_RESPONSE_KEYS) },
  { method: 'POST', segments: ['import-package'], body: { kind: 'multipart' }, response: json(SHARE_IMPORT_KEYS) },
  { method: 'GET', segments: ['reports'], query: ['team_id', ...PAGING], response: json(['reports']) },
  { method: 'POST', segments: ['reports'], body: fields(['team_id', ...REPORT_FIELDS]), response: json(REPORT_KEYS) },
  { method: 'GET', segments: ['reports', UUID], response: json(REPORT_KEYS) },
  { method: 'PATCH', segments: ['reports', UUID], body: fields(REPORT_FIELDS), response: json(REPORT_KEYS) },
  { method: 'DELETE', segments: ['reports', UUID], response: { kind: 'empty' } },
  { method: 'POST', segments: ['reports', UUID, 'export'], body: fields(SHARE_EXPORT_FIELDS), response: { kind: 'download' } },
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
  { method: 'GET', segments: ['players', STEAM_ID, 'positions'], query: PLAYER_POSITIONS_QUERY, response: json(PLAYER_POSITIONS_KEYS) },
  { method: 'GET', segments: ['players', STEAM_ID, 'metrics'], query: PLAYER_METRICS_QUERY, response: json(METRICS_KEYS) },
  { method: 'GET', segments: ['baseline'], query: BASELINE_QUERY, response: json(BASELINE_KEYS) },
  { method: 'GET', segments: ['leaderboard'], query: LEADERBOARD_QUERY, response: json(LEADERBOARD_KEYS) },
  { method: 'GET', segments: ['norms'], query: NORMS_QUERY, response: json(NORMS_KEYS) },
  { method: 'GET', segments: ['pinned-players'], response: json(PINNED_PLAYERS_KEYS) },
  { method: 'PUT', segments: ['pinned-players'], body: fields(PINNED_PLAYERS_FIELDS), response: json(PINNED_PLAYERS_KEYS) },
  { method: 'GET', segments: ['pro-lineups'], response: json(PRO_LINEUP_MAPS_KEYS) },
  { method: 'GET', segments: ['pro-lineups', MAP], response: json(PRO_LINEUP_MAP_KEYS) },
  { method: 'GET', segments: ['pro-lineups', 'clips', PRO_CLIP_MATCH, PRO_CLIP_FILE], response: { kind: 'stream' } },
  { method: 'POST', segments: ['pro-lineups', MAP, 'lineups', PRO_LINEUP_ID, 'save'], response: json(LINEUP_KEYS) },
  { method: 'GET', segments: ['pro-matches'], query: PRO_MATCH_QUERY, response: json(PRO_MATCHES_KEYS) },
  { method: 'POST', segments: ['pro-matches', HLTV_ID, PRO_ENTRY, 'import'], response: json(PRO_IMPORT_KEYS) },
  { method: 'GET', segments: ['pro-follows'], response: json(['follows']) },
  { method: 'POST', segments: ['pro-follows'], body: fields(['team_id', 'name']), response: json(PRO_FOLLOW_KEYS) },
  { method: 'DELETE', segments: ['pro-follows', HLTV_ID], response: { kind: 'empty' } },
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
