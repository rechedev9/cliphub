import { jsonInit, request } from './library.ts';

// Browser-side contract of Partidas pro (`internal/httpapi/library_pro_matches.go`):
// recent HLTV matches the ClipHub server analysed, imported map by map without the demo.

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
  /** One per demo; a server restart splits a map, and Studio joins the parts. */
  parts: { entry: string; first_round: number; rounds: number }[];
  /** The analysis is older than the current scan, so some numbers may be missing. */
  old_analysis: boolean;
  /** The library match once the map is imported. */
  library_match_id?: string;
  /** The analysis is older than the map's radar calibration: the 2D may draw positions off. */
  radar_may_be_off: boolean;
};

/** hltv_url is the match page on HLTV, where its demos can be downloaded. */
export type ProMatch = { id: number; event: string; played_at: string; hltv_url: string; teams: ProMatchTeam[]; maps: ProMatchMap[] };

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
  old_analysis: boolean;
  hltv_url: string;
};

export type ProMatchQuery = { days: number; team: string; event: string; map: string; page: number };

/** since is the local midnight proMatchSearch sends for Hoy. */
export const PRO_MATCH_QUERY = ['days', 'since', 'team', 'event', 'map', 'page'] as const satisfies readonly (keyof ProMatchQuery | 'since')[];
export const PRO_MATCHES_KEYS = [
  'status', 'updated_at', 'days', 'page', 'pages', 'total', 'matches', 'facets', 'follows',
] as const satisfies readonly (keyof ProMatchesPage)[];
export const PRO_IMPORT_KEYS = ['match_id', 'demo_id', 'created', 'studio_path', 'radar_may_be_off'] as const satisfies readonly (keyof ProImport)[];
export const PRO_FOLLOW_KEYS = ['team_id', 'name', 'followed_at', 'synced_at'] as const satisfies readonly (keyof ProFollow)[];
export const PRO_MATCH_INFO_KEYS = [
  'hltv_match_id', 'entry', 'map', 'event', 'played_at', 'teams', 'analysis_version', 'radar_may_be_off', 'old_analysis', 'hltv_url',
] as const satisfies readonly (keyof ProMatchInfo)[];

/** The periods the page offers, in days. */
export const PRO_PERIODS = [
  { days: 1, label: 'Hoy' },
  { days: 7, label: '7 días' },
  { days: 30, label: '30 días' },
  { days: 90, label: '90 días' },
] as const;

export const DEFAULT_PRO_QUERY: ProMatchQuery = { days: 7, team: '', event: '', map: '', page: 1 };

/** The query string of a catalog read, without the defaults. Hoy starts at the reader's midnight. */
export function proMatchSearch(q: ProMatchQuery, now: Date = new Date()): string {
  const params = new URLSearchParams();
  if (q.days !== DEFAULT_PRO_QUERY.days) params.set('days', String(q.days));
  if (q.days === 1) params.set('since', new Date(now.getFullYear(), now.getMonth(), now.getDate()).toISOString());
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

/** The local calendar day of a time, as YYYY-MM-DD. */
export function localDay(value: string | Date): string {
  const d = new Date(value);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

/** A day heading: Hoy, Ayer or the local date. */
export function proDayLabel(day: string, now: Date = new Date()): string {
  const yesterday = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1);
  if (day === localDay(now)) return 'Hoy';
  if (day === localDay(yesterday)) return 'Ayer';
  const [y, m, d] = day.split('-').map(Number);
  const date = new Date(y ?? 0, (m ?? 1) - 1, d ?? 1);
  const year = date.getFullYear() === now.getFullYear() ? {} : { year: 'numeric' as const };
  return new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', ...year }).format(date);
}

/** The matches grouped by the local day they were played, newest day first, keeping the server's order. */
export function groupByDay(matches: readonly ProMatch[]): { day: string; matches: ProMatch[] }[] {
  const groups: { day: string; matches: ProMatch[] }[] = [];
  for (const m of matches) {
    const day = localDay(m.played_at);
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

/** Says a map was played over several demos, which Studio joins; null for one demo. */
export function proMapDemos(map: Pick<ProMatchMap, 'parts'>): string | null {
  return map.parts.length > 1 ? `${map.parts.length} demos unidas` : null;
}
