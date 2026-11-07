export const STUDIO_MATCH_MODE_CHANNEL = 'cliphub:match-mode';

/** The web route of the compact window, and its query keys (LIBRETA_QUERY in web/lib/clips/routes.ts). */
const MATCH_MODE_PATH = '/libreta/partida';
const MAP_PATTERN = /^[a-z0-9_]{1,64}$/;
const TEAM_PATTERN = /^(none|[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})$/;

export interface MatchModeRequest {
  map: string;
  side: 'T' | 'CT' | '';
  team: string;
}

/** Parses the only Modo partida message accepted from the sandboxed preload. */
export function parseMatchModeRequest(value: unknown): MatchModeRequest {
  if (!isRecord(value) || Object.keys(value).some((key) => key !== 'map' && key !== 'side' && key !== 'team')) {
    throw new Error('invalid match mode request');
  }
  const { map, side = '', team = '' } = value;
  if (typeof map !== 'string' || !MAP_PATTERN.test(map)) throw new Error('match mode needs a map');
  if (side !== '' && side !== 'T' && side !== 'CT') throw new Error('side must be T or CT');
  if (typeof team !== 'string' || (team !== '' && !TEAM_PATTERN.test(team))) throw new Error('team must be a team id');
  return { map, side, team };
}

/** The page the window loads on the web origin. */
export function matchModeUrl(origin: string, request: MatchModeRequest): string {
  const url = new URL(MATCH_MODE_PATH, origin);
  url.searchParams.set('mapa', request.map);
  if (request.side !== '') url.searchParams.set('lado', request.side);
  if (request.team !== '') url.searchParams.set('equipo', request.team);
  return url.href;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}
