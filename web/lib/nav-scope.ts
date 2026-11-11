import { SCOPE_PARAMS } from './analysis/scope.ts';
import type { SidebarEntry } from './nav-groups.ts';

/** The query key of the team on the tool pages (Replay 2D, Buscador de patrones, Editor de tácticas); the Libreta uses it too. */
export const TEAM_QUERY = 'equipo';

const SCOPE_NAMES: ReadonlySet<string> = new Set(Object.values(SCOPE_PARAMS));
const TEAM_PATH = /^\/(?:equipos|patrones)\/([^/]+)/;

export type PageLocation = { pathname: string; search: string };

/** The team the page is about: a team record carries it in the path, the tool pages in `equipo`. */
export function scopeTeamId(where: PageLocation): string | null {
  const fromPath = TEAM_PATH.exec(where.pathname)?.[1];
  if (fromPath !== undefined) return decodeURIComponent(fromPath);
  const fromQuery = new URLSearchParams(where.search).get(TEAM_QUERY);
  return fromQuery === null || fromQuery === '' ? null : fromQuery;
}

/** The analysis scope (map, side, period, matches, buys, filters) the page keeps in its query. */
function scopeQuery(search: string): URLSearchParams {
  const out = new URLSearchParams();
  for (const [name, value] of new URLSearchParams(search)) if (SCOPE_NAMES.has(name)) out.append(name, value);
  return out;
}

function withQuery(path: string, params: URLSearchParams): string {
  const query = params.toString();
  return query === '' ? path : `${path}?${query}`;
}

/**
 * Where a rail entry goes from this page. With a team in scope, the tools that read a team keep it and
 * the scope (map, side, period, matches, buys) as EDGE does; without one they open their own door.
 */
export function sidebarHref(entry: SidebarEntry, where: PageLocation): string {
  const team = entry.carry === undefined ? null : scopeTeamId(where);
  if (entry.carry === undefined || team === null) return entry.href;
  const scope = scopeQuery(where.search);
  const tool = new URLSearchParams([[TEAM_QUERY, team], ...scope]);
  const record = `/equipos/${encodeURIComponent(team)}`;
  switch (entry.carry) {
    case 'replay':
      return withQuery('/replay', tool);
    case 'patrones':
      return withQuery(`/patrones/${encodeURIComponent(team)}`, scope);
    case 'tacticas':
      return withQuery('/tacticas/nueva', tool);
    case 'team-stats':
      return withQuery(record, scope);
    case 'playbook':
      return withQuery(record, new URLSearchParams([['tab', 'playbook'], ...scope]));
    case 'prep':
      return withQuery(record, new URLSearchParams([['tab', 'preparar'], ...scope]));
    case 'veto':
      return withQuery(record, new URLSearchParams([['tab', 'veto']]));
  }
}

/** The same page for another team: the scope stays, the rival and the lineup belong to the team they were set on. */
export function withTeam(where: PageLocation, teamId: string): string {
  const params = new URLSearchParams(where.search);
  params.set(TEAM_QUERY, teamId);
  params.delete(SCOPE_PARAMS.opponent);
  params.delete(SCOPE_PARAMS.lineup);
  return `${where.pathname}?${params.toString()}`;
}

/** The team record tabs the rail opens on the hub (`/equipos?tab=`) before a team is chosen. */
const HUB_TABS: ReadonlySet<string> = new Set(['playbook', 'preparar', 'veto']);

/** A team's record from the hub: the tab the rail asked for stays preselected. */
export function hubTeamHref(teamId: string, tab: string | null): string {
  const record = `/equipos/${encodeURIComponent(teamId)}`;
  return tab !== null && HUB_TABS.has(tab) ? `${record}?tab=${tab}` : record;
}
