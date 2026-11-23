import type { SearchKind, SearchResult } from '../api/search.ts';
import { GRENADE_KIND_LABEL } from '../analysis/grenade-patterns.ts';
import { isLineupGrenade } from '../api/lineups.ts';
import { isLibretaStatus } from '../api/libreta.ts';
import { LIBRETA_TABS, LONG_VIDEOS_HREF, SHORTS_HREF, libretaTabHref, lineupHref, matchHref, playerHref, playHref, teamHref } from '../clips/routes.ts';
import { navSectionsFor } from '../edition.ts';
import { currentEdition, isPathAvailable, type Edition } from '../edition-routes.ts';
import { STATUS_LABEL } from '../libreta/status.ts';
import { playKindLabel } from '../stratbook/plays.ts';

/** The section titles of the result list, in the order they show. */
export const SEARCH_GROUP_LABEL: Record<SearchGroup, string> = {
  action: 'Acciones',
  section: 'Secciones',
  play: 'Jugadas',
  lineup: 'Lineups',
  call: 'Llamadas',
  team: 'Equipos',
  player: 'Jugadores',
  match: 'Partidas',
};

type SearchGroup = SearchKind | 'section' | 'action';

export type SearchRow = { key: string; group: SearchGroup; title: string; subtitle: string; href: string };

/** Mirrors `library.FoldSearch`: lower case, no accents, "&" read as "y". */
export function foldSearch(text: string): string {
  return text
    .normalize('NFD')
    .replace(/\p{Mn}/gu, '')
    .toLocaleLowerCase('es')
    .replaceAll('&', ' y ')
    .split(/\s+/)
    .filter((word) => word !== '')
    .join(' ');
}

type SearchAction = { title: string; keywords: string; subtitle: string; href: string; webHref?: string };

const SEARCH_ACTIONS: readonly SearchAction[] = [
  { title: 'Crear Short', keywords: 'vertical video nuevo', subtitle: '', href: SHORTS_HREF },
  { title: 'Crear demo completa', keywords: 'video largo partida entera nueva', subtitle: '', href: LONG_VIDEOS_HREF },
  { title: 'Importar demos', keywords: 'cargar subir demo dem', subtitle: 'Desde Partidas', href: '/partidas', webHref: '/importar' },
  { title: 'Crear clips de stream', keywords: 'stream twitch youtube kick recortar', subtitle: '', href: '/streams' },
  { title: 'Abrir ajustes', keywords: 'configuracion settings steam voz', subtitle: '', href: '/settings' },
];

function matchesWords(text: string, query: string): boolean {
  const words = foldSearch(query).split(' ').filter((word) => word !== '');
  const folded = foldSearch(text);
  return words.every((word) => folded.includes(word));
}

function actionHref(action: SearchAction, edition: Edition): string {
  return edition === 'web' && action.webHref !== undefined ? action.webHref : action.href;
}

export function actionRows(query: string, edition: Edition = currentEdition()): SearchRow[] {
  return SEARCH_ACTIONS.flatMap((action) => {
    const href = actionHref(action, edition);
    if (!isPathAvailable(href, edition) || !matchesWords(`${action.title} ${action.keywords}`, query)) return [];
    return [{ key: `action:${action.title}`, group: 'action', title: action.title, subtitle: action.subtitle, href }];
  });
}

/** The sections of the rail whose name holds every word of the query. */
export function sectionRows(query: string): SearchRow[] {
  if (foldSearch(query) === '') return [];
  return navSectionsFor(currentEdition()).filter((section) => matchesWords(section.label, query)).map((section) => ({
    key: `section:${section.href}`,
    group: 'section',
    title: section.label,
    subtitle: 'Ir a la sección',
    href: section.href,
  }));
}

export function localRows(query: string): SearchRow[] {
  const actions = actionRows(query);
  const opened = new Set(actions.map((row) => row.href));
  return [...actions, ...sectionRows(query).filter((row) => !opened.has(row.href))];
}

function sideLabel(side: string | undefined): string | null {
  if (side === 'both') return 'T y CT';
  return side === 'T' || side === 'CT' ? side : null;
}

function statusLabel(status: string | undefined): string | null {
  return status !== undefined && isLibretaStatus(status) && status !== 'active' ? STATUS_LABEL[status] : null;
}

function mapLabel(map: string | undefined): string | null {
  return map === undefined ? null : map.replace(/^de_/, '').replace(/^./, (c) => c.toLocaleUpperCase('es'));
}

const MATCH_DAY = new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', year: 'numeric', timeZone: 'UTC' });

/** "2026-11-07" from the server as "7 nov 2026"; anything else passes through. */
function dayLabel(day: string): string {
  const time = /^\d{4}-\d{2}-\d{2}$/.test(day) ? Date.parse(`${day}T00:00:00Z`) : Number.NaN;
  return Number.isNaN(time) ? day : MATCH_DAY.format(time);
}

function hrefFor(result: SearchResult): string {
  switch (result.kind) {
    case 'play':
      return playHref(result.id);
    case 'lineup':
      return lineupHref(result.id);
    case 'call':
      return libretaTabHref(LIBRETA_TABS.calls);
    case 'team':
      return teamHref(result.id);
    case 'player':
      return playerHref(result.id);
    case 'match':
      return matchHref(result.id);
  }
}

function detailFor(result: SearchResult): string | null {
  const detail = result.detail ?? '';
  switch (result.kind) {
    case 'play':
      return detail === '' ? null : playKindLabel(detail);
    case 'lineup':
      return isLineupGrenade(detail) ? GRENADE_KIND_LABEL[detail] : null;
    case 'call':
      return detail === '' ? null : detail.split('\n')[0] ?? null;
    case 'team':
    case 'player':
      return detail === '' ? null : `${detail} ${detail === '1' ? 'partida' : 'partidas'}`;
    case 'match':
      return detail === '' ? null : dayLabel(detail);
  }
}

/** A server result as a row: where it goes and a short line of what it is. */
export function resultRow(result: SearchResult): SearchRow {
  const parts = [detailFor(result), mapLabel(result.map), sideLabel(result.side), result.team_name ?? null, statusLabel(result.status)];
  return {
    key: `${result.kind}:${result.id}`,
    group: result.kind,
    title: result.title,
    subtitle: parts.filter((part): part is string => part !== null && part !== '').join(' · '),
    href: hrefFor(result),
  };
}
