/**
 * Hrefs of the creation flows and the results area. One builder per screen so
 * a route rename is a single edit, and query keys are named constants rather
 * than strings repeated across pages.
 */

export const SHORTS_HREF = '/shorts' as const;
export const LONG_VIDEOS_HREF = '/videos-largos' as const;
export const RESULTS_HREF = '/mis-videos' as const;
export const MATCHES_HREF = '/partidas' as const;
export const TEAMS_HREF = '/equipos' as const;
export const PLAYERS_HREF = '/jugadores' as const;
export const REPORTS_HREF = '/informes' as const;
export const STRATBOOK_HREF = '/stratbook' as const;
/** The FACEIT follow list, under Jugadores beside the library players. */
export const FACEIT_PLAYERS_HREF = '/jugadores/faceit' as const;

export const PRODUCE_FORMAT = {
  short: 'short',
  full: 'full',
} as const;
export type ProduceFormat = (typeof PRODUCE_FORMAT)[keyof typeof PRODUCE_FORMAT];

/** Each format owns a flow: its home loads the demo, `<home>/<id>` prepares the video. */
export const FLOW_HREF = {
  [PRODUCE_FORMAT.short]: SHORTS_HREF,
  [PRODUCE_FORMAT.full]: LONG_VIDEOS_HREF,
} as const satisfies Record<ProduceFormat, string>;

/** `?job=` resumes a `scanned` job's POV pick instead of uploading. */
export const FLOW_QUERY = { job: 'job' } as const;

const JOB_ID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** True when a `?job=` value is a well-formed orchestrator job id; anything else is a bad/guessed URL. */
export function isJobIdParam(value: string | string[] | null | undefined): value is string {
  return typeof value === 'string' && JOB_ID_RE.test(value);
}

/** Home of one flow; with `job` it resumes that demo's player pick. */
export function flowHref(format: ProduceFormat, opts: { job?: string } = {}): string {
  const base = FLOW_HREF[format];
  if (opts.job === undefined) return base;
  const params = new URLSearchParams({ [FLOW_QUERY.job]: opts.job });
  return `${base}?${params.toString()}`;
}

/**
 * `?series=` returns to the bo3/bo5 series after producing. `?rondas=` comes
 * from the analysis area: the video covers only those source rounds.
 */
export const PRODUCE_QUERY = { series: 'series', rounds: 'rondas' } as const;

/** The most rounds a selection carries (`recapplan.MaxSelectedRounds`). */
export const MAX_SELECTED_ROUNDS = 60;

/** The `?rondas=` value: increasing, unique round numbers from 1 to 999, or null when absent or invalid. */
export function parseRoundsParam(value: string | string[] | null | undefined): number[] | null {
  if (typeof value !== 'string' || value === '') return null;
  const rounds: number[] = [];
  for (const part of value.split(',')) {
    if (!/^[1-9][0-9]{0,2}$/.test(part)) return null;
    rounds.push(Number(part));
  }
  const unique = [...new Set(rounds)].sort((a, b) => a - b);
  return unique.length > MAX_SELECTED_ROUNDS ? null : unique;
}

/** A flow's producer for a job, limited to some rounds of the match. */
export function roundsProduceHref(jobId: string, format: ProduceFormat, rounds: readonly number[]): string {
  const base = `${FLOW_HREF[format]}/${encodeURIComponent(jobId)}`;
  if (rounds.length === 0) return base;
  const params = new URLSearchParams({ [PRODUCE_QUERY.rounds]: [...rounds].sort((a, b) => a - b).join(',') });
  return `${base}?${params.toString()}`;
}

export function produceHref(matchId: string, format: ProduceFormat, seriesId?: string): string {
  const base = `${FLOW_HREF[format]}/${encodeURIComponent(matchId)}`;
  if (seriesId === undefined || seriesId === '') return base;
  const params = new URLSearchParams({ [PRODUCE_QUERY.series]: seriesId });
  return `${base}?${params.toString()}`;
}

/** Query keys that open Partidas already filtered, such as a team's matches on one map. */
export const MATCHES_QUERY = { team: 'team', map: 'map' } as const;

export function matchesHref(filters: { team?: string; map?: string } = {}): string {
  const params = new URLSearchParams();
  if (filters.team) params.set(MATCHES_QUERY.team, filters.team);
  if (filters.map) params.set(MATCHES_QUERY.map, filters.map);
  const query = params.toString();
  return query === '' ? MATCHES_HREF : `${MATCHES_HREF}?${query}`;
}

/** A partida's record: its tactical analysis, rounds and replay. */
export function matchHref(matchId: string): string {
  return `${MATCHES_HREF}/${encodeURIComponent(matchId)}`;
}

/** A library team's record. */
export function teamHref(teamId: string): string {
  return `${TEAMS_HREF}/${encodeURIComponent(teamId)}`;
}

/** A saved scouting report. */
export function reportHref(reportId: string): string {
  return `${REPORTS_HREF}/${encodeURIComponent(reportId)}`;
}

/** A saved stratbook play. */
export function playHref(playId: string): string {
  return `${STRATBOOK_HREF}/${encodeURIComponent(playId)}`;
}

/** Query keys that open the stratbook on one team's plays, or on the detection tab. */
export const STRATBOOK_QUERY = { team: 'equipo', tab: 'tab' } as const;
export const STRATBOOK_DETECT_TAB = 'detectar';

export function teamStratbookHref(teamId: string, detect = false): string {
  const params = new URLSearchParams({ [STRATBOOK_QUERY.team]: teamId });
  if (detect) params.set(STRATBOOK_QUERY.tab, STRATBOOK_DETECT_TAB);
  return `${STRATBOOK_HREF}?${params.toString()}`;
}

/** Query keys that open Informes with the new-report dialog on one team. */
export const REPORTS_QUERY = { team: 'equipo' } as const;

export function newReportHref(teamId: string): string {
  return `${REPORTS_HREF}?${new URLSearchParams({ [REPORTS_QUERY.team]: teamId }).toString()}`;
}

/** A library player's record, by SteamID64. */
export function playerHref(steamId: string): string {
  return `${PLAYERS_HREF}/${encodeURIComponent(steamId)}`;
}

/** A library series: its maps, its two teams and its known veto. */
export function librarySeriesHref(seriesId: string): string {
  return `${MATCHES_HREF}/series/${encodeURIComponent(seriesId)}`;
}

export const LIBRARY_SERIES_HREF = `${MATCHES_HREF}/series` as const;

export function seriesHref(seriesId: string): string {
  return `/series/${encodeURIComponent(seriesId)}`;
}

export function publishHref(matchId: string, clipId: string): string {
  return `${RESULTS_HREF}/${encodeURIComponent(matchId)}/publicar/${encodeURIComponent(clipId)}`;
}

/** Orphan reels (no `jobId`) still publish; the segment is a placeholder. */
export const ORPHAN_MATCH_SEGMENT = '-' as const;
