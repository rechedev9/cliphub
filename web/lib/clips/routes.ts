/**
 * Hrefs of the creation flows and the results area. One builder per screen so
 * a route rename is a single edit, and query keys are named constants rather
 * than strings repeated across pages.
 */

export const SHORTS_HREF = '/shorts' as const;
export const STREAMS_HREF = '/streams' as const;
export const LONG_VIDEOS_HREF = '/videos-largos' as const;
export const RESULTS_HREF = '/mis-videos' as const;
export const MATCHES_HREF = '/partidas' as const;
export const TEAMS_HREF = '/equipos' as const;
/** Player sheets live under this prefix; the list itself is retired. */
const PLAYERS_HREF = '/jugadores' as const;
export const REPORTS_HREF = '/informes' as const;
export const LIBRETA_HREF = '/libreta' as const;
/** Jugadores FACEIT, the follow list in the Crear area of the rail. */
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

/** A partida's record opened on its 2D Replay. */
export function matchReplayHref(matchId: string): string {
  return `${matchHref(matchId)}?tab=replay`;
}

/** A library team's record. */
export function teamHref(teamId: string): string {
  return `${TEAMS_HREF}/${encodeURIComponent(teamId)}`;
}

/** A saved scouting report. */
export function reportHref(reportId: string): string {
  return `${REPORTS_HREF}/${encodeURIComponent(reportId)}`;
}

/** A saved play of the Libreta. */
export function playHref(playId: string): string {
  return `${LIBRETA_HREF}/${encodeURIComponent(playId)}`;
}

/** A saved lineup of the Libreta. */
export function lineupHref(lineupId: string): string {
  return `${LIBRETA_HREF}/lineups/${encodeURIComponent(lineupId)}`;
}

/** The Libreta's tabs, by their `?tab=` value. */
export const LIBRETA_TABS = {
  plays: 'jugadas',
  detect: 'detectar',
  table: 'tabla',
  lineups: 'lineups',
  roles: 'roles',
  calls: 'llamadas',
  callouts: 'callouts',
} as const;
export type LibretaTab = (typeof LIBRETA_TABS)[keyof typeof LIBRETA_TABS];

/** Query keys that open the Libreta on one tab, team or map. */
export const LIBRETA_QUERY = { team: 'equipo', tab: 'tab', map: 'mapa', side: 'lado' } as const;
export const LIBRETA_DETECT_TAB = LIBRETA_TABS.detect;

export function libretaTabHref(tab: LibretaTab, params: Readonly<Record<string, string>> = {}): string {
  const search = new URLSearchParams({ [LIBRETA_QUERY.tab]: tab, ...params });
  return `${LIBRETA_HREF}?${search.toString()}`;
}

export function teamLibretaHref(teamId: string, detect = false): string {
  const params = new URLSearchParams({ [LIBRETA_QUERY.team]: teamId });
  if (detect) params.set(LIBRETA_QUERY.tab, LIBRETA_DETECT_TAB);
  return `${LIBRETA_HREF}?${params.toString()}`;
}

/** Query keys that open Informes with the new-report dialog on one team. */
export const REPORTS_QUERY = { team: 'equipo' } as const;

export function newReportHref(teamId: string): string {
  return `${REPORTS_HREF}?${new URLSearchParams({ [REPORTS_QUERY.team]: teamId }).toString()}`;
}

/** Query keys of the printable rival prep: team id, map, and optional days `YYYY-MM-DD`. */
export const PREP_QUERY = { team: 'team', map: 'map', from: 'from', to: 'to', lineup: 'lineup' } as const;

/** A team's Preparar tab, on one map when it is known. */
export function teamPrepareHref(teamId: string, map = ''): string {
  const params = new URLSearchParams({ tab: 'preparar' });
  if (map !== '') params.set('map', map);
  return `${teamHref(teamId)}?${params.toString()}`;
}

/** The printable prep of a rival on one map; `lineup` is the SteamID64s it counts, none for every round. */
export function prepHref(prep: { teamId: string; map: string; from?: string; to?: string; lineup?: readonly string[] }): string {
  const params = new URLSearchParams({ [PREP_QUERY.team]: prep.teamId, [PREP_QUERY.map]: prep.map });
  if (prep.from) params.set(PREP_QUERY.from, prep.from);
  if (prep.to) params.set(PREP_QUERY.to, prep.to);
  if (prep.lineup !== undefined && prep.lineup.length > 0) params.set(PREP_QUERY.lineup, prep.lineup.join(','));
  return `${REPORTS_HREF}/prep?${params.toString()}`;
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

/** The segment under /videos-largos that holds a Demo completa series. */
export const FULL_SERIES_SEGMENT = 'serie' as const;

/** A bo3/bo5 series; from Demo completa it lives under that flow and each map prepares its full demo. */
export function seriesHref(seriesId: string, format: ProduceFormat): string {
  const id = encodeURIComponent(seriesId);
  return format === PRODUCE_FORMAT.full ? `${LONG_VIDEOS_HREF}/${FULL_SERIES_SEGMENT}/${id}` : `/series/${id}`;
}

export function publishHref(matchId: string, clipId: string): string {
  return `${RESULTS_HREF}/${encodeURIComponent(matchId)}/publicar/${encodeURIComponent(clipId)}`;
}

/** Orphan reels (no `jobId`) still publish; the segment is a placeholder. */
export const ORPHAN_MATCH_SEGMENT = '-' as const;
