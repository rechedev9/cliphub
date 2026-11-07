import type { SearchKind, SearchResult } from '../api/search.ts';
import { GRENADE_KIND_LABEL } from '../analysis/grenade-patterns.ts';
import { isLineupGrenade } from '../api/lineups.ts';
import { isLibretaStatus } from '../api/libreta.ts';
import { LIBRETA_TABS, libretaTabHref, lineupHref, matchHref, playerHref, playHref, teamHref } from '../clips/routes.ts';
import { STATUS_LABEL } from '../libreta/status.ts';
import { NAV_SECTIONS } from '../nav.ts';
import { playKindLabel } from '../stratbook/plays.ts';

/** The section titles of the result list, in the order they show. */
export const SEARCH_GROUP_LABEL: Record<SearchKind | 'section', string> = {
  section: 'Secciones',
  play: 'Jugadas',
  lineup: 'Lineups',
  call: 'Llamadas',
  team: 'Equipos',
  player: 'Jugadores',
  match: 'Partidas',
};

export type SearchRow = { key: string; group: SearchKind | 'section'; title: string; subtitle: string; href: string };

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

/** The sections of the rail whose name holds every word of the query. */
export function sectionRows(query: string): SearchRow[] {
  const words = foldSearch(query).split(' ').filter((word) => word !== '');
  if (words.length === 0) return [];
  return NAV_SECTIONS.filter((section) => words.every((word) => foldSearch(section.label).includes(word))).map((section) => ({
    key: `section:${section.href}`,
    group: 'section',
    title: section.label,
    subtitle: 'Ir a la sección',
    href: section.href,
  }));
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
