import type { MetricsReport } from './metrics.ts';
import type { LibraryReport, ReportSide, ScoutReport } from './scout.ts';
import type { Annotation, HeatmapGrenade, HeatmapKind, SpatialResponse, Stroke } from './spatial.ts';
import type {
  LibraryPlay,
  LibraryPlayDetail,
  PlayDetection,
  PlayInput,
  PlayPatch,
  RoundLineups,
  StratbookFile,
  StratbookImport,
  StratbookUpload,
} from './stratbook.ts';
import type { TacticalBuyType } from './tactical.ts';
import { SERVICE_UNAVAILABLE_CODE } from './types.ts';
import { vetoSearch, type LibrarySeries, type SeriesInput, type TeamMaps, type VetoQuery, type VetoSimulation } from './veto.ts';

/**
 * Browser-side contract of the shared demo library (`internal/library`). Every
 * type mirrors a Go struct field for field with its JSON names; the Go package
 * is the source of truth. All access is same-origin through `/api/library/*`.
 */

export const LIBRARY_SOURCES = ['valve', 'faceit', 'hltv', 'scrim', 'unknown'] as const;
export type LibrarySource = (typeof LIBRARY_SOURCES)[number];

export const LIBRARY_PARSE_STATES = ['queued', 'parsing', 'ready', 'failed'] as const;
export type LibraryParseState = (typeof LIBRARY_PARSE_STATES)[number];

export const LIBRARY_TEAM_KEYS = ['ct-start', 't-start'] as const;
export type LibraryTeamKey = (typeof LIBRARY_TEAM_KEYS)[number];

export type LibraryAssignment = 'auto' | 'manual' | 'none';

export type LibraryDemo = {
  id: string;
  sha256: string;
  file_name: string;
  path: string;
  size_bytes: number;
  source: LibrarySource;
  source_manual: boolean;
  cs2_build?: number;
  patch_version?: number;
  state: LibraryParseState;
  error_code?: string;
  error?: string;
  tags: string[];
  /** Crear jobs made from this demo, oldest first: the match's videos hang off them. */
  job_ids: string[];
  match_id?: string;
  played_at?: string;
  imported_at: string;
  updated_at: string;
};

export type LibraryMatchTeam = {
  key: LibraryTeamKey;
  start_side: 'CT' | 'T';
  clan_name: string;
  team_id: string | null;
  team_name?: string;
  assignment: LibraryAssignment;
  score: number;
};

/** One player's line, summed over the rounds of the match's tactical document. */
export type LibraryMatchPlayer = {
  steamid64: string;
  name: string;
  team_key: LibraryTeamKey;
  slot: number;
  rounds: number;
  kills: number;
  deaths: number;
  assists: number;
  damage: number;
};

export type LibraryMatch = {
  id: string;
  demo_id: string;
  series_id?: string;
  map: string;
  played_at: string;
  source: LibrarySource;
  tickrate: number;
  duration_ticks: number;
  round_count: number;
  teams: LibraryMatchTeam[];
  players?: LibraryMatchPlayer[];
  created_at: string;
  updated_at: string;
};

export type LibraryRosterEntry = {
  steamid64: string;
  nick: string;
  first_played_at: string;
  last_played_at: string;
  matches: number;
};

export type LibraryTeam = {
  id: string;
  name: string;
  aliases: string[];
  match_count: number;
  roster?: LibraryRosterEntry[];
  created_at: string;
  updated_at: string;
};

export type LibraryPlayerNick = { nick: string; first_seen_at: string; last_seen_at: string };

export type LibraryPlayerTeam = {
  team_id: string;
  team_name: string;
  first_played_at: string;
  last_played_at: string;
  matches: number;
};

export type LibraryPlayer = {
  steamid64: string;
  nick: string;
  nicks?: LibraryPlayerNick[];
  faceit_player_id?: string;
  faceit_nickname?: string;
  match_count: number;
  teams?: LibraryPlayerTeam[];
  created_at: string;
  updated_at: string;
};

export const LIBRARY_ROLES = ['awp', 'entry', 'support', 'rifler', 'unknown'] as const;
export type LibraryRole = (typeof LIBRARY_ROLES)[number];

/** One round a role metric counted (`library.RoleEvidence`). */
export type LibraryRoleEvidence = { match_id: string; round: number; tick: number };

export type LibraryRoleMetric = { count: number; total: number; examples: LibraryRoleEvidence[] };

/** A player's inferred role and the counts it was read from (`library.PlayerRole`). */
export type LibraryPlayerRole = {
  steamid64: string;
  role: LibraryRole;
  match_count: number;
  rounds: number;
  awp_kills: LibraryRoleMetric;
  t_openings: LibraryRoleMetric;
  utility: LibraryRoleMetric;
  team_utility: LibraryRoleMetric;
  thresholds: {
    min_rounds: number;
    awp_kill_share: number;
    entry_opening_share: number;
    support_utility_ratio: number;
  };
};

export type LibraryImportResult = {
  file_name: string;
  status: 'imported' | 'duplicate' | 'rejected';
  demo?: LibraryDemo;
  error_code?: string;
  error?: string;
};

/** Error code of a job or library demo whose file is no longer stored. */
export const JOB_DEMO_MISSING_CODE = 'job_demo_missing';

/** Page size of the list endpoints (`library.MaxListLimit`). */
const LIBRARY_PAGE = 200;
/** The most pages a list read follows, so a runaway listing cannot loop forever. */
const LIBRARY_MAX_PAGES = 25;

const LIBRARY_API_BASE = '/api/library';

export class LibraryApiError extends Error {
  readonly status: number;
  readonly code?: string;

  constructor(message: string, status: number, code?: string) {
    super(message);
    this.name = 'LibraryApiError';
    this.status = status;
    this.code = code;
  }
}

/** True when the library could not be read because the local service is down. */
export function isLibraryOffline(error: unknown): boolean {
  return error instanceof LibraryApiError && error.code === SERVICE_UNAVAILABLE_CODE;
}

/** True for a 404 from the library, which the pages read as "not in the library". */
export function isLibraryNotFound(error: unknown): boolean {
  return error instanceof LibraryApiError && error.status === 404;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${LIBRARY_API_BASE}${path}`, { cache: 'no-store', ...init });
  if (!res.ok) {
    const body = (await res.json().catch(() => null)) as { error?: unknown; code?: unknown } | null;
    const message = body && typeof body.error === 'string' ? body.error : `library request failed (${res.status})`;
    throw new LibraryApiError(message, res.status, body && typeof body.code === 'string' ? body.code : undefined);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

function jsonInit(method: string, body?: unknown): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  };
}

function id(value: string): string {
  return encodeURIComponent(value);
}

/** Reads every page of a list endpoint into one array, in the server's order. */
async function readAll<T>(path: string, key: string, params: URLSearchParams = new URLSearchParams()): Promise<T[]> {
  const out: T[] = [];
  for (let page = 0; page < LIBRARY_MAX_PAGES; page++) {
    const query = new URLSearchParams(params);
    query.set('limit', String(LIBRARY_PAGE));
    query.set('offset', String(page * LIBRARY_PAGE));
    const body = await request<Record<string, T[] | undefined>>(`${path}?${query.toString()}`);
    const items = body[key] ?? [];
    out.push(...items);
    if (items.length < LIBRARY_PAGE) break;
  }
  return out;
}

export function listLibraryDemos(): Promise<LibraryDemo[]> {
  return readAll<LibraryDemo>('/demos', 'demos');
}

export function getLibraryDemo(demoId: string): Promise<LibraryDemo> {
  return request<LibraryDemo>(`/demos/${id(demoId)}`);
}

export function patchLibraryDemo(demoId: string, patch: { source?: LibrarySource; tags?: string[] }): Promise<LibraryDemo> {
  return request<LibraryDemo>(`/demos/${id(demoId)}`, jsonInit('PATCH', patch));
}

export function analyzeLibraryDemo(demoId: string): Promise<LibraryDemo> {
  return request<LibraryDemo>(`/demos/${id(demoId)}/analyze`, { method: 'POST' });
}

export function deleteLibraryDemo(demoId: string): Promise<void> {
  return request<void>(`/demos/${id(demoId)}`, { method: 'DELETE' });
}

/** Starts a Crear job over a library demo and returns its job id. */
export async function createJobFromLibraryDemo(demoId: string): Promise<string> {
  const body = await request<{ job_id: string }>(`/demos/${id(demoId)}/jobs`, { method: 'POST' });
  return body.job_id;
}

export type PlayerJob = { job_id: string; status: string; reused: boolean };

/**
 * A Crear job that parses one player's POV of a library demo right away. A job
 * that already parses that player and made no video yet is returned again.
 */
export function createPlayerJob(demoId: string, steamId: string): Promise<PlayerJob> {
  return request<PlayerJob>(`/demos/${id(demoId)}/jobs`, jsonInit('POST', { target_steamid: steamId }));
}

/** Imports one demo file; the page sends files one by one to show progress per file. */
export async function importLibraryDemo(file: File, tags: readonly string[] = []): Promise<LibraryImportResult> {
  const form = new FormData();
  for (const tag of tags) form.append('tag', tag);
  form.append('demo', file, file.name);
  const body = await request<{ demos: LibraryImportResult[] }>('/demos', { method: 'POST', body: form });
  const result = body.demos[0];
  if (result === undefined) throw new LibraryApiError('the import returned no result', 502);
  return result;
}

export type LibraryMatchQuery = { team_id?: string; steamid?: string; map?: string; series_id?: string };

export function listLibraryMatches(query: LibraryMatchQuery = {}): Promise<LibraryMatch[]> {
  const params = new URLSearchParams();
  if (query.team_id) params.set('team_id', query.team_id);
  if (query.steamid) params.set('steamid', query.steamid);
  if (query.map) params.set('map', query.map);
  if (query.series_id) params.set('series_id', query.series_id);
  return readAll<LibraryMatch>('/matches', 'matches', params);
}

export function getLibraryMatch(matchId: string): Promise<LibraryMatch> {
  return request<LibraryMatch>(`/matches/${id(matchId)}`);
}

/** Moves a match to a series (or out of it with null), or corrects its date. */
export function patchLibraryMatch(matchId: string, patch: { series_id?: string | null; played_at?: string }): Promise<LibraryMatch> {
  return request<LibraryMatch>(`/matches/${id(matchId)}`, jsonInit('PATCH', patch));
}

export function listLibrarySeries(query: { team_id?: string } = {}): Promise<LibrarySeries[]> {
  const params = new URLSearchParams();
  if (query.team_id) params.set('team_id', query.team_id);
  return readAll<LibrarySeries>('/series', 'series', params);
}

export function getLibrarySeries(seriesId: string): Promise<LibrarySeries> {
  return request<LibrarySeries>(`/series/${id(seriesId)}`);
}

export function createLibrarySeries(input: SeriesInput): Promise<LibrarySeries> {
  return request<LibrarySeries>('/series', jsonInit('POST', input));
}

export function patchLibrarySeries(seriesId: string, patch: SeriesInput): Promise<LibrarySeries> {
  return request<LibrarySeries>(`/series/${id(seriesId)}`, jsonInit('PATCH', patch));
}

export function deleteLibrarySeries(seriesId: string): Promise<void> {
  return request<void>(`/series/${id(seriesId)}`, { method: 'DELETE' });
}

/** A team's results, side rounds and known vetoes on every map. */
export function getLibraryTeamMaps(teamId: string): Promise<TeamMaps> {
  return request<TeamMaps>(`/teams/${id(teamId)}/maps`);
}

/** A simulated veto against a rival: the given steps kept, the rest recommended. */
export function getLibraryTeamVeto(teamId: string, query: VetoQuery): Promise<VetoSimulation> {
  return request<VetoSimulation>(`/teams/${id(teamId)}/veto?${vetoSearch(query)}`);
}

export type TeamAssignmentInput = { team_id?: string; team_name?: string; remember_alias?: boolean };

export function assignLibraryMatchTeam(matchId: string, key: LibraryTeamKey, input: TeamAssignmentInput): Promise<LibraryMatch> {
  return request<LibraryMatch>(`/matches/${id(matchId)}/teams/${key}`, jsonInit('PUT', input));
}

export function listLibraryTeams(): Promise<LibraryTeam[]> {
  return readAll<LibraryTeam>('/teams', 'teams');
}

export function getLibraryTeam(teamId: string): Promise<LibraryTeam> {
  return request<LibraryTeam>(`/teams/${id(teamId)}`);
}

export function patchLibraryTeam(teamId: string, patch: { name?: string; aliases?: string[] }): Promise<LibraryTeam> {
  return request<LibraryTeam>(`/teams/${id(teamId)}`, jsonInit('PATCH', patch));
}

export function mergeLibraryTeam(teamId: string, intoTeamId: string): Promise<LibraryTeam> {
  return request<LibraryTeam>(`/teams/${id(teamId)}/merge`, jsonInit('POST', { into_team_id: intoTeamId }));
}

export function listLibraryPlayers(): Promise<LibraryPlayer[]> {
  return readAll<LibraryPlayer>('/players', 'players');
}

export function getLibraryPlayer(steamId: string): Promise<LibraryPlayer> {
  return request<LibraryPlayer>(`/players/${id(steamId)}`);
}

export function linkLibraryPlayerFaceit(steamId: string): Promise<LibraryPlayer> {
  return request<LibraryPlayer>(`/players/${id(steamId)}/faceit`, { method: 'POST' });
}

export function getLibraryPlayerRole(steamId: string): Promise<LibraryPlayerRole> {
  return request<LibraryPlayerRole>(`/players/${id(steamId)}/role`);
}

/** The library demo a Crear job was made from, or null when the job is not linked yet. */
export async function getLibraryJobDemo(jobId: string): Promise<LibraryDemo | null> {
  try {
    return await request<LibraryDemo>(`/jobs/${id(jobId)}`);
  } catch (error) {
    if (isLibraryNotFound(error)) return null;
    throw error;
  }
}

export function linkLibraryJob(jobId: string): Promise<LibraryDemo> {
  return request<LibraryDemo>(`/jobs/${id(jobId)}`, { method: 'POST' });
}

/** Recent Crear jobs whose demo is not in the library yet. */
export async function listUnlinkedLibraryJobs(): Promise<string[]> {
  const body = await request<{ unlinked_job_ids: string[] }>('/jobs');
  return body.unlinked_job_ids;
}

export async function linkAllLibraryJobs(): Promise<LibraryDemo[]> {
  const body = await request<{ demos: LibraryDemo[] }>('/jobs', { method: 'POST' });
  return body.demos;
}

export function listLibraryReports(): Promise<LibraryReport[]> {
  return readAll<LibraryReport>('/reports', 'reports');
}

export function getLibraryReport(reportId: string): Promise<LibraryReport> {
  return request<LibraryReport>(`/reports/${id(reportId)}`);
}

export type ReportInput = { title: string; team_id: string; match_ids: string[]; side: ReportSide; notes?: string };

export function createLibraryReport(input: ReportInput): Promise<LibraryReport> {
  return request<LibraryReport>('/reports', jsonInit('POST', input));
}

export type ReportPatch = { title?: string; match_ids?: string[]; side?: ReportSide; notes?: string };

export function patchLibraryReport(reportId: string, patch: ReportPatch): Promise<LibraryReport> {
  return request<LibraryReport>(`/reports/${id(reportId)}`, jsonInit('PATCH', patch));
}

export function deleteLibraryReport(reportId: string): Promise<void> {
  return request<void>(`/reports/${id(reportId)}`, { method: 'DELETE' });
}

/** A saved report's sections, computed from its matches on every read. */
export function getLibraryReportScout(reportId: string): Promise<ScoutReport> {
  return request<ScoutReport>(`/reports/${id(reportId)}/scout`);
}

/** Every player's advanced metrics in a match, with the win probability of each round. */
export function getLibraryMatchMetrics(matchId: string): Promise<MetricsReport> {
  return request<MetricsReport>(`/matches/${id(matchId)}/metrics`);
}

/** The advanced metrics of the scouted team's players over a report's matches. */
export function getLibraryReportMetrics(reportId: string): Promise<MetricsReport> {
  return request<MetricsReport>(`/reports/${id(reportId)}/metrics`);
}

export type PlayerMetricsQuery = { map?: string; side?: ReportSide; limit?: number };

export function playerMetricsPath(steamId: string, query: PlayerMetricsQuery): string {
  const params = new URLSearchParams();
  if (query.map) params.set('map', query.map);
  if (query.side) params.set('side', query.side);
  if (query.limit) params.set('limit', String(query.limit));
  const search = params.toString();
  return `/players/${id(steamId)}/metrics${search ? `?${search}` : ''}`;
}

/** A player's advanced metrics over their most recent matches. */
export function getLibraryPlayerMetrics(steamId: string, query: PlayerMetricsQuery = {}): Promise<MetricsReport> {
  return request<MetricsReport>(playerMetricsPath(steamId, query));
}

export type TeamScoutQuery = {
  maps?: readonly string[];
  side?: ReportSide;
  from?: string;
  to?: string;
  opponent_ids?: readonly string[];
  match_ids?: readonly string[];
};

/** A team's scouting report over its matches, filtered by map, side, date and rival. */
export function getLibraryTeamScout(teamId: string, query: TeamScoutQuery = {}): Promise<ScoutReport> {
  const params = new URLSearchParams();
  for (const map of query.maps ?? []) params.append('map', map);
  if (query.side) params.set('side', query.side);
  if (query.from) params.set('from', query.from);
  if (query.to) params.set('to', query.to);
  for (const opponent of query.opponent_ids ?? []) params.append('opponent_id', opponent);
  for (const match of query.match_ids ?? []) params.append('match_id', match);
  const search = params.toString();
  return request<ScoutReport>(`/teams/${id(teamId)}/scout${search === '' ? '' : `?${search}`}`);
}

/** Whose rounds a heatmap or an overlay reads: one match with the tactical filter, or a team across matches. */
export type SpatialScope =
  | { kind: 'match'; matchId: string; /** The tactical filter query (`tacticalFilterToQuery`). */ filter: string }
  | { kind: 'team'; teamId: string; query: TeamSpatialQuery };

export type TeamSpatialQuery = {
  map?: string;
  side?: ReportSide;
  from?: string;
  opponent_ids?: readonly string[];
  match_ids?: readonly string[];
  buys?: readonly TacticalBuyType[];
};

export type HeatmapQuery = { kind: HeatmapKind; from?: number; to?: number; grenades?: readonly HeatmapGrenade[] };

/** The path of a scope's heatmap or overlay with its query, in a stable order. */
export function spatialPath(scope: SpatialScope, view: 'heatmap' | 'overlay', extra: URLSearchParams): string {
  const params = new URLSearchParams(scope.kind === 'match' ? scope.filter : '');
  if (scope.kind === 'team') {
    const q = scope.query;
    if (q.map) params.set('map', q.map);
    if (q.side) params.set('side', q.side);
    if (q.from) params.set('from', q.from);
    for (const opponent of q.opponent_ids ?? []) params.append('opponent_id', opponent);
    for (const match of q.match_ids ?? []) params.append('match_id', match);
    for (const buy of q.buys ?? []) params.append('buy', buy);
  }
  for (const [key, value] of extra) params.append(key, value);
  const search = params.toString();
  const base = scope.kind === 'match' ? `/matches/${id(scope.matchId)}/tactical/${view}` : `/teams/${id(scope.teamId)}/${view}`;
  return search === '' ? base : `${base}?${search}`;
}

/** A heatmap of the scope's rounds, with the radar it is drawn on. */
export function getSpatialHeatmap(scope: SpatialScope, heat: HeatmapQuery): Promise<SpatialResponse> {
  const extra = new URLSearchParams({ kind: heat.kind });
  if (heat.from !== undefined && heat.from > 0) extra.set('t_from', String(heat.from));
  if (heat.to !== undefined && heat.to > 0) extra.set('t_to', String(heat.to));
  for (const grenade of heat.grenades ?? []) extra.append('grenade', grenade);
  return request<SpatialResponse>(spatialPath(scope, 'heatmap', extra));
}

/** The scope's rounds laid on one clock from freeze end. */
export function getSpatialOverlay(scope: SpatialScope, limit?: number): Promise<SpatialResponse> {
  const extra = new URLSearchParams();
  if (limit !== undefined) extra.set('limit', String(limit));
  return request<SpatialResponse>(spatialPath(scope, 'overlay', extra));
}

/** The notes of up to 50 matches, by match, round and tick. */
export async function listAnnotations(matchIds: readonly string[], round?: number): Promise<Annotation[]> {
  if (matchIds.length === 0) return [];
  const params = new URLSearchParams();
  for (const match of matchIds) params.append('match_id', match);
  if (round !== undefined) params.set('round', String(round));
  const body = await request<{ annotations?: Annotation[] }>(`/annotations?${params.toString()}`);
  return body.annotations ?? [];
}

export type AnnotationInput = { match_id: string; round: number; tick: number; text: string; strokes: Stroke[] };

export function createAnnotation(input: AnnotationInput): Promise<Annotation> {
  return request<Annotation>('/annotations', jsonInit('POST', input));
}

export function patchAnnotation(annotationId: string, patch: { tick?: number; text?: string; strokes?: Stroke[] }): Promise<Annotation> {
  return request<Annotation>(`/annotations/${id(annotationId)}`, jsonInit('PATCH', patch));
}

export function deleteAnnotation(annotationId: string): Promise<void> {
  return request<void>(`/annotations/${id(annotationId)}`, { method: 'DELETE' });
}

export type PlayListQuery = { team_id?: string; map?: string; side?: ReportSide; kind?: string };

function playListParams(query: PlayListQuery): URLSearchParams {
  const params = new URLSearchParams();
  if (query.team_id) params.set('team_id', query.team_id);
  if (query.map) params.set('map', query.map);
  if (query.side) params.set('side', query.side);
  if (query.kind) params.set('kind', query.kind);
  return params;
}

/** The stratbook's plays, by most recent edit. */
export function listLibraryPlays(query: PlayListQuery = {}): Promise<LibraryPlay[]> {
  return readAll<LibraryPlay>('/plays', 'plays', playListParams(query));
}

export function getLibraryPlay(playId: string): Promise<LibraryPlayDetail> {
  return request<LibraryPlayDetail>(`/plays/${id(playId)}`);
}

export function createLibraryPlay(input: PlayInput): Promise<LibraryPlay> {
  return request<LibraryPlay>('/plays', jsonInit('POST', input));
}

export function patchLibraryPlay(playId: string, patch: PlayPatch): Promise<LibraryPlay> {
  return request<LibraryPlay>(`/plays/${id(playId)}`, jsonInit('PATCH', patch));
}

export function deleteLibraryPlay(playId: string): Promise<void> {
  return request<void>(`/plays/${id(playId)}`, { method: 'DELETE' });
}

/** The draft of the play one side's team ran in a round, with positions read at `tick`. */
export async function getRoundPlayDraft(matchId: string, round: number, teamKey: LibraryTeamKey, tick?: number): Promise<LibraryPlay> {
  const params = new URLSearchParams({ team_key: teamKey });
  if (tick !== undefined) params.set('tick', String(Math.round(tick)));
  const body = await request<{ draft: LibraryPlay }>(`/matches/${id(matchId)}/tactical/rounds/${round}/play?${params.toString()}`);
  return body.draft;
}

/** A round's grenades compared with the bundled smoke lineup catalog. */
export function getRoundLineups(matchId: string, round: number): Promise<RoundLineups> {
  return request<RoundLineups>(`/matches/${id(matchId)}/tactical/rounds/${round}/lineups`);
}

/** The plays a team repeats across its matches, grouped by utility and positions. */
export function detectTeamPlays(teamId: string, query: TeamScoutQuery = {}): Promise<PlayDetection> {
  const params = new URLSearchParams();
  for (const map of query.maps ?? []) params.append('map', map);
  if (query.side) params.set('side', query.side);
  if (query.from) params.set('from', query.from);
  if (query.to) params.set('to', query.to);
  for (const opponent of query.opponent_ids ?? []) params.append('opponent_id', opponent);
  for (const match of query.match_ids ?? []) params.append('match_id', match);
  const search = params.toString();
  return request<PlayDetection>(`/teams/${id(teamId)}/plays${search === '' ? '' : `?${search}`}`);
}

export function exportStratbook(query: PlayListQuery = {}): Promise<StratbookFile> {
  const search = playListParams(query).toString();
  return request<StratbookFile>(`/plays/export${search === '' ? '' : `?${search}`}`);
}

export function importStratbook(file: StratbookUpload): Promise<StratbookImport> {
  return request<StratbookImport>('/plays/import', jsonInit('POST', file));
}
