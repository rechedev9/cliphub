/**
 * Hrefs of the creation flows and the results area. One builder per screen so
 * a route rename is a single edit, and query keys are named constants rather
 * than strings repeated across pages.
 */

export const SHORTS_HREF = '/shorts' as const;
export const LONG_VIDEOS_HREF = '/videos-largos' as const;
export const RESULTS_HREF = '/resultados' as const;
export const CLOUD_HREF = '/nube' as const;

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

/** `?series=` returns to the bo3/bo5 series after producing. */
export const PRODUCE_QUERY = { series: 'series' } as const;

export function produceHref(matchId: string, format: ProduceFormat, seriesId?: string): string {
  const base = `${FLOW_HREF[format]}/${encodeURIComponent(matchId)}`;
  if (seriesId === undefined || seriesId === '') return base;
  const params = new URLSearchParams({ [PRODUCE_QUERY.series]: seriesId });
  return `${base}?${params.toString()}`;
}

/** `?origen=` narrows the results to what one place recorded. */
export const RESULTS_QUERY = { origin: 'origen' } as const;

export const RESULTS_ORIGIN = {
  local: 'pc',
  cloud: 'nube',
} as const;
export type ResultsOrigin = (typeof RESULTS_ORIGIN)[keyof typeof RESULTS_ORIGIN];

/** Resolves `?origen=`; a missing, repeated or unknown value shows every result. */
export function resultsOriginParam(value: string | string[] | null | undefined): ResultsOrigin | null {
  return value === RESULTS_ORIGIN.local || value === RESULTS_ORIGIN.cloud ? value : null;
}

export function resultsHref(opts: { origin?: ResultsOrigin } = {}): string {
  if (opts.origin === undefined) return RESULTS_HREF;
  const params = new URLSearchParams({ [RESULTS_QUERY.origin]: opts.origin });
  return `${RESULTS_HREF}?${params.toString()}`;
}

export function seriesHref(seriesId: string): string {
  return `/series/${encodeURIComponent(seriesId)}`;
}

export function publishHref(matchId: string, clipId: string): string {
  return `${RESULTS_HREF}/${encodeURIComponent(matchId)}/publicar/${encodeURIComponent(clipId)}`;
}

/** Orphan reels (no `jobId`) still publish; the segment is a placeholder. */
export const ORPHAN_MATCH_SEGMENT = '-' as const;
