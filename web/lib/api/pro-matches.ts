import { jsonInit, request } from './library.ts';

/**
 * Browser-side contract of Partidas pro (`internal/httpapi/library_pro_matches.go`):
 * recent HLTV matches the ClipHub server analysed, imported map by map without the demo.
 */

/** ok: the server answered; disabled: no community server; missing: it has no catalog yet; unavailable: it did not answer. */
export type ProCatalogStatus = 'ok' | 'disabled' | 'missing' | 'unavailable';

export type ProMatchTeam = { id: number; name: string; rank?: number; followed: boolean };

export type ProMatchMap = {
  entry: string;
  map: string;
  rounds: number;
  analysis_version: number;
  bytes: number;
  sha256: string;
  /** The library match once the map is imported. */
  library_match_id?: string;
  /** The analysis is older than the map's radar calibration: the 2D may draw positions off. */
  radar_may_be_off: boolean;
};

export type ProMatch = { id: number; event: string; played_at: string; teams: ProMatchTeam[]; maps: ProMatchMap[] };

export type ProTeamCount = { id: number; name: string; rank?: number; matches: number };
export type ProNameCount = { name: string; matches: number };

export type ProFollow = { team_id: number; name: string; followed_at: string; synced_at?: string };

export type ProMatchesPage = {
  status: ProCatalogStatus;
  updated_at?: string;
  days: number;
  page: number;
  pages: number;
  total: number;
  matches: ProMatch[];
  facets: { teams: ProTeamCount[]; events: ProNameCount[]; maps: ProNameCount[] };
  follows: ProFollow[];
};

export type ProImport = { match_id: string; demo_id: string; created: boolean; studio_path: string; radar_may_be_off: boolean };

/** The HLTV match a pro import came from, for the match page. */
export type ProMatchInfo = {
  hltv_match_id: number;
  entry: string;
  map: string;
  event: string;
  played_at: string;
  teams: { id: number; name: string; rank?: number }[];
  analysis_version: number;
  radar_may_be_off: boolean;
};

export type ProMatchQuery = { days: number; team: string; event: string; map: string; page: number };

export const PRO_MATCH_QUERY = ['days', 'team', 'event', 'map', 'page'] as const satisfies readonly (keyof ProMatchQuery)[];
export const PRO_MATCHES_KEYS = [
  'status', 'updated_at', 'days', 'page', 'pages', 'total', 'matches', 'facets', 'follows',
] as const satisfies readonly (keyof ProMatchesPage)[];
export const PRO_IMPORT_KEYS = ['match_id', 'demo_id', 'created', 'studio_path', 'radar_may_be_off'] as const satisfies readonly (keyof ProImport)[];
export const PRO_FOLLOW_KEYS = ['team_id', 'name', 'followed_at', 'synced_at'] as const satisfies readonly (keyof ProFollow)[];
export const PRO_MATCH_INFO_KEYS = [
  'hltv_match_id', 'entry', 'map', 'event', 'played_at', 'teams', 'analysis_version', 'radar_may_be_off',
] as const satisfies readonly (keyof ProMatchInfo)[];

/** The periods the page offers, in days. */
export const PRO_PERIODS = [
  { days: 1, label: 'Hoy' },
  { days: 7, label: '7 días' },
  { days: 30, label: '30 días' },
  { days: 90, label: '90 días' },
] as const;

export const DEFAULT_PRO_QUERY: ProMatchQuery = { days: 7, team: '', event: '', map: '', page: 1 };

/** The query string of a catalog read, without the defaults. */
export function proMatchSearch(q: ProMatchQuery): string {
  const params = new URLSearchParams();
  if (q.days !== DEFAULT_PRO_QUERY.days) params.set('days', String(q.days));
  if (q.team) params.set('team', q.team);
  if (q.event) params.set('event', q.event);
  if (q.map) params.set('map', q.map);
  if (q.page > 1) params.set('page', String(q.page));
  const s = params.toString();
  return s ? `?${s}` : '';
}

export function listProMatches(q: ProMatchQuery): Promise<ProMatchesPage> {
  return request<ProMatchesPage>(`/pro-matches${proMatchSearch(q)}`);
}

export function importProMatch(matchId: number, entry: string): Promise<ProImport> {
  return request<ProImport>(`/pro-matches/${matchId}/${encodeURIComponent(entry)}/import`, jsonInit('POST'));
}

export function followProTeam(teamId: number, name: string): Promise<ProFollow> {
  return request<ProFollow>('/pro-follows', jsonInit('POST', { team_id: teamId, name }));
}

export function unfollowProTeam(teamId: number): Promise<void> {
  return request<void>(`/pro-follows/${teamId}`, { method: 'DELETE' });
}

export function getMatchPro(matchId: string): Promise<ProMatchInfo> {
  return request<ProMatchInfo>(`/matches/${encodeURIComponent(matchId)}/pro`);
}

/** What the page says when the catalog cannot be read; null when it can. */
export function proCatalogNotice(status: ProCatalogStatus): { title: string; description: string } | null {
  switch (status) {
    case 'disabled':
      return {
        title: 'El servidor de ClipHub está desactivado',
        description: 'Este Studio está configurado para no conectarse al servidor de ClipHub, de donde vienen las partidas pro.',
      };
    case 'missing':
      return {
        title: 'Aún no hay partidas pro',
        description: 'El servidor de ClipHub todavía no publica partidas pro. Aparecerán aquí en cuanto se actualice; no tienes que hacer nada.',
      };
    case 'unavailable':
      return {
        title: 'El servidor de ClipHub no responde',
        description: 'No se pudieron leer las partidas pro. Revisa tu conexión y vuelve a intentarlo en un rato.',
      };
    default:
      return null;
  }
}

/** The matches grouped by the day they were played, newest day first, keeping the server's order. */
export function groupByDay(matches: readonly ProMatch[]): { day: string; matches: ProMatch[] }[] {
  const groups: { day: string; matches: ProMatch[] }[] = [];
  for (const m of matches) {
    const day = m.played_at.slice(0, 10);
    const last = groups.at(-1);
    if (last && last.day === day) last.matches.push(m);
    else groups.push({ day, matches: [m] });
  }
  return groups;
}

/** "Vitality vs FaZe", or the event when the match names no teams. */
export function proMatchTitle(m: Pick<ProMatch, 'teams' | 'event'>): string {
  return m.teams.length === 2 ? `${m.teams[0]?.name} vs ${m.teams[1]?.name}` : m.event;
}

/** A map's part label: a server restart splits one map into parts p1, p2. */
export function proMapPart(entry: string): string | null {
  const part = /-p([0-9]+)$/.exec(entry);
  return part ? `parte ${part[1]}` : null;
}
