/** The three finders the Buscador de patrones hosts, one panel each. */
export const PATTERN_MODES = ['granadas', 'jugadores', 'patron'] as const;
export type PatternMode = (typeof PATTERN_MODES)[number];

export const PATTERN_MODE_LABEL: Record<PatternMode, string> = { granadas: 'Granadas', jugadores: 'Jugadores', patron: 'Patrón' };

/** Query key of the open panel; Granadas is the default and is left out of the URL. */
export const PATTERN_MODE_QUERY = 'modo';
export const DEFAULT_PATTERN_MODE: PatternMode = 'granadas';

/** The forms of the team Replay, so a link to one of them opens the matching panel. */
const REPLAY_FORM_MODE: Readonly<Record<string, PatternMode>> = { granadas: 'granadas', calor: 'jugadores', patrones: 'patron' };

export function patternModeFromQuery(query: Pick<URLSearchParams, 'get'>): PatternMode {
  const asked = query.get(PATTERN_MODE_QUERY);
  const own = PATTERN_MODES.find((mode) => mode === asked);
  if (own !== undefined) return own;
  return REPLAY_FORM_MODE[query.get('forma') ?? ''] ?? DEFAULT_PATTERN_MODE;
}

/** The query with the panel written over it, every other key kept; the Replay's own `forma` goes, since it no longer applies. */
export function writePatternMode(current: string, mode: PatternMode): string {
  const params = new URLSearchParams(current);
  params.delete('forma');
  if (mode === DEFAULT_PATTERN_MODE) params.delete(PATTERN_MODE_QUERY);
  else params.set(PATTERN_MODE_QUERY, mode);
  return params.toString();
}

/** The Buscador de patrones; with a team id, that team's canvas. */
export const PATTERNS_HREF = '/patrones' as const;

export function patternsHref(teamId: string): string {
  return teamId === '' ? PATTERNS_HREF : `${PATTERNS_HREF}/${encodeURIComponent(teamId)}`;
}
