/**
 * Hrefs of the 01 Clips y vídeos section. One builder per screen so a route
 * rename is a single edit, and query keys are named constants rather than
 * strings repeated across pages.
 */

export const CLIPS_HREF = '/clips' as const;
export const NEW_DEMO_HREF = '/clips/nueva' as const;

/** `?vista=` picks the hub lens; `?partida=` opens one row. */
export const HUB_QUERY = {
  lens: 'vista',
  open: 'partida',
} as const;

export const HUB_LENS = {
  matches: 'partidas',
  clips: 'clips',
} as const;
export type HubLens = (typeof HUB_LENS)[keyof typeof HUB_LENS];

export function isHubLens(value: string | null): value is HubLens {
  return value === HUB_LENS.matches || value === HUB_LENS.clips;
}

/** `?formato=` picks the output; `?series=` returns to the bo3/bo5 series after producing. */
export const PRODUCE_QUERY = { format: 'formato', series: 'series' } as const;

export const PRODUCE_FORMAT = {
  short: 'short',
  full: 'full',
} as const;
export type ProduceFormat = (typeof PRODUCE_FORMAT)[keyof typeof PRODUCE_FORMAT];

export function isProduceFormat(value: string | null): value is ProduceFormat {
  return value === PRODUCE_FORMAT.short || value === PRODUCE_FORMAT.full;
}

export function hubHref(opts: { lens?: HubLens; open?: string } = {}): string {
  const params = new URLSearchParams();
  if (opts.lens !== undefined && opts.lens !== HUB_LENS.matches) params.set(HUB_QUERY.lens, opts.lens);
  if (opts.open !== undefined) params.set(HUB_QUERY.open, opts.open);
  const query = params.toString();
  return query === '' ? CLIPS_HREF : `${CLIPS_HREF}?${query}`;
}

export function produceHref(
  matchId: string,
  format: ProduceFormat = PRODUCE_FORMAT.short,
  seriesId?: string,
): string {
  const params = new URLSearchParams();
  if (format !== PRODUCE_FORMAT.short) params.set(PRODUCE_QUERY.format, format);
  if (seriesId !== undefined && seriesId !== '') params.set(PRODUCE_QUERY.series, seriesId);
  const base = `${CLIPS_HREF}/${encodeURIComponent(matchId)}/nuevo`;
  const query = params.toString();
  return query === '' ? base : `${base}?${query}`;
}

export function seriesHref(seriesId: string): string {
  return `/series/${encodeURIComponent(seriesId)}`;
}

export function publishHref(matchId: string, clipId: string): string {
  return `${CLIPS_HREF}/${encodeURIComponent(matchId)}/publicar/${encodeURIComponent(clipId)}`;
}

/** Orphan reels (no `jobId`) still publish; the segment is a placeholder. */
export const ORPHAN_MATCH_SEGMENT = '-' as const;
