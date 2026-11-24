import { isPathAvailable, type Edition } from './edition-routes.ts';
import { navSection, sectionForPath, type NavHref } from './nav.ts';

export type SidebarEntryId =
  | 'demo-completa'
  | 'short'
  | 'clips-de-stream'
  | 'mis-videos'
  | 'ideas'
  | 'partidas'
  | 'replay'
  | 'buscador-de-patrones'
  | 'equipos'
  | 'jugadores'
  | 'jugadores-faceit'
  | 'ranking'
  | 'libreta'
  | 'editor-de-tacticas'
  | 'lineups-pro'
  | 'informes'
  | 'almacenamiento'
  | 'ajustes';

/** How an entry keeps the team and the analysis scope of the page the user is on (see `sidebarHref`). */
export type ScopeCarry = 'replay' | 'patrones' | 'tacticas' | 'team-stats';

export type SidebarEntry = {
  id: SidebarEntryId;
  label: string;
  /** Where the entry goes while no team is in scope. */
  href: `/${string}`;
  carry?: ScopeCarry;
};

export type SidebarGroup = { id: string; label: string; entries: readonly SidebarEntry[] };

function railEntry(id: SidebarEntryId, href: NavHref, carry?: ScopeCarry): SidebarEntry {
  const { label } = navSection(href);
  return carry === undefined ? { id, label, href } : { id, label, href, carry };
}

/** Rail groups in order. Group labels render as `/ LABEL`. */
export const SIDEBAR_GROUPS: readonly SidebarGroup[] = [
  {
    id: 'crear',
    label: 'Crear',
    entries: [
      railEntry('demo-completa', '/videos-largos'),
      railEntry('short', '/shorts'),
      railEntry('clips-de-stream', '/streams'),
      railEntry('mis-videos', '/mis-videos'),
      railEntry('ideas', '/ideas'),
    ],
  },
  {
    id: 'partidas',
    label: 'Partidas',
    entries: [
      railEntry('partidas', '/partidas'),
      railEntry('replay', '/replay', 'replay'),
      railEntry('buscador-de-patrones', '/patrones', 'patrones'),
    ],
  },
  {
    id: 'equipos-y-jugadores',
    label: 'Estadísticas',
    entries: [
      railEntry('equipos', '/equipos', 'team-stats'),
      railEntry('jugadores', '/jugadores'),
      railEntry('jugadores-faceit', '/jugadores/faceit'),
      railEntry('ranking', '/ranking'),
    ],
  },
  {
    id: 'preparacion',
    label: 'Preparación',
    entries: [
      railEntry('libreta', '/libreta'),
      railEntry('editor-de-tacticas', '/tacticas/nueva', 'tacticas'),
      railEntry('lineups-pro', '/lineups'),
      railEntry('informes', '/informes'),
    ],
  },
];

/** The two entries docked to the footer under the Demo and Voz buttons. */
export const SIDEBAR_FOOTER_ENTRIES: readonly SidebarEntry[] = [
  railEntry('almacenamiento', '/almacenamiento'),
  railEntry('ajustes', '/settings'),
];

function entryPath(href: string): string {
  return href.split('?')[0];
}

export function sidebarGroupsFor(edition: Edition): readonly SidebarGroup[] {
  return SIDEBAR_GROUPS
    .map((group) => ({
      ...group,
      entries: group.entries.filter((entry) => isPathAvailable(entryPath(entry.href), edition)),
    }))
    .filter((group) => group.entries.length > 0);
}

export function sidebarFooterEntriesFor(edition: Edition): readonly SidebarEntry[] {
  return SIDEBAR_FOOTER_ENTRIES.filter((entry) => isPathAvailable(entryPath(entry.href), edition));
}

export const VOICE_SETTINGS_ANCHOR = 'voz-del-equipo';

export const VOICE_HREF = `/settings#${VOICE_SETTINGS_ANCHOR}`;

function under(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

export function activeEntryId(where: { pathname: string }): SidebarEntryId | null {
  const { pathname } = where;
  if (under(pathname, '/replay/playlists')) return 'partidas';
  const section = sectionForPath(pathname);
  if (section === null) return null;
  const all = [...SIDEBAR_GROUPS.flatMap((group) => group.entries), ...SIDEBAR_FOOTER_ENTRIES];
  return all.find((entry) => entry.href === section.href)?.id ?? null;
}
