import { sectionForPath } from './nav.ts';

export type SidebarEntryId =
  | 'demo-completa'
  | 'short'
  | 'clips-de-stream'
  | 'jugadores-faceit'
  | 'mis-videos'
  | 'ideas'
  | 'partidas'
  | 'replay'
  | 'buscador-de-patrones'
  | 'estadisticas-de-equipo'
  | 'estadisticas-de-jugador'
  | 'ranking'
  | 'editor-de-tacticas'
  | 'playbook'
  | 'lineups-pro'
  | 'simulacion-de-veto'
  | 'preparar-rival'
  | 'almacenamiento'
  | 'ajustes';

/** How an entry keeps the team and the analysis scope of the page the user is on (see `sidebarHref`). */
export type ScopeCarry = 'replay' | 'patrones' | 'tacticas' | 'team-stats' | 'playbook' | 'prep' | 'veto';

export type SidebarEntry = {
  id: SidebarEntryId;
  label: string;
  /** Where the entry goes while no team is in scope. */
  href: `/${string}`;
  carry?: ScopeCarry;
};

export type SidebarGroup = { id: string; label: string; entries: readonly SidebarEntry[] };

/** Rail groups in order. Group labels render as `/ LABEL`. */
export const SIDEBAR_GROUPS: readonly SidebarGroup[] = [
  {
    id: 'crear',
    label: 'Crear',
    entries: [
      { id: 'demo-completa', label: 'Demo completa', href: '/videos-largos' },
      { id: 'short', label: 'Short', href: '/shorts' },
      { id: 'clips-de-stream', label: 'Clips de stream', href: '/streams' },
      { id: 'jugadores-faceit', label: 'Jugadores FACEIT', href: '/jugadores/faceit' },
      { id: 'mis-videos', label: 'Mis vídeos', href: '/mis-videos' },
      { id: 'ideas', label: 'Ideas', href: '/ideas' },
    ],
  },
  {
    id: 'partidas',
    label: 'Partidas',
    entries: [{ id: 'partidas', label: 'Partidas', href: '/partidas' }],
  },
  {
    id: 'herramientas-2d',
    label: 'Herramientas 2D',
    entries: [
      { id: 'replay', label: 'Replay 2D', href: '/replay', carry: 'replay' },
      { id: 'buscador-de-patrones', label: 'Buscador de patrones', href: '/patrones', carry: 'patrones' },
    ],
  },
  {
    id: 'estadisticas',
    label: 'Estadísticas',
    entries: [
      { id: 'estadisticas-de-equipo', label: 'Estadísticas de equipo', href: '/equipos', carry: 'team-stats' },
      { id: 'estadisticas-de-jugador', label: 'Estadísticas de jugador', href: '/jugadores' },
      { id: 'ranking', label: 'Ranking', href: '/ranking' },
    ],
  },
  {
    id: 'playbook',
    label: 'Playbook',
    entries: [
      { id: 'editor-de-tacticas', label: 'Editor de tácticas', href: '/tacticas/nueva', carry: 'tacticas' },
      { id: 'playbook', label: 'Playbook', href: '/equipos?tab=playbook', carry: 'playbook' },
      { id: 'lineups-pro', label: 'Lineups pro', href: '/lineups' },
    ],
  },
  {
    id: 'otras-herramientas',
    label: 'Más',
    entries: [
      { id: 'simulacion-de-veto', label: 'Simulación de veto', href: '/equipos?tab=veto', carry: 'veto' },
      { id: 'preparar-rival', label: 'Preparar rival', href: '/equipos?tab=preparar', carry: 'prep' },
    ],
  },
];

/** The two entries docked to the footer under the Demo and Voz buttons. */
export const SIDEBAR_FOOTER_ENTRIES: readonly SidebarEntry[] = [
  { id: 'almacenamiento', label: 'Almacenamiento', href: '/almacenamiento' },
  { id: 'ajustes', label: 'Ajustes', href: '/settings' },
];

/** Where the Voz button goes: TeamSpeak recordings are set up in Ajustes; the Replay voice row imports per match. */
export const VOICE_HREF = '/settings';

function under(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

/** The team record's tabs that are tools of their own in the rail; `scouting` is the old name of Preparar. */
const TEAM_TAB_ENTRY: Readonly<Record<string, SidebarEntryId>> = {
  playbook: 'playbook',
  preparar: 'preparar-rival',
  scouting: 'preparar-rival',
  veto: 'simulacion-de-veto',
};

/**
 * The one entry of the rail that lights for a page. A tool that lives inside a section
 * (the team record's Playbook, Preparar and Veto tabs, the player stats) lights its own row;
 * every other page lights the entry of its section.
 */
export function activeEntryId(where: { pathname: string; tab: string | null }): SidebarEntryId | null {
  const { pathname, tab } = where;
  // Playlists is a Partidas tab that happens to live under /replay.
  if (under(pathname, '/replay/playlists')) return 'partidas';
  if (under(pathname, '/replay')) return 'replay';
  if (under(pathname, '/patrones')) return 'buscador-de-patrones';
  if (under(pathname, '/tacticas')) return 'editor-de-tacticas';
  if (under(pathname, '/equipos')) return (tab === null ? undefined : TEAM_TAB_ENTRY[tab]) ?? 'estadisticas-de-equipo';
  if (under(pathname, '/jugadores') && !under(pathname, '/jugadores/faceit')) return 'estadisticas-de-jugador';
  // Tools reached from inside another screen light that screen's row.
  if (under(pathname, '/libreta')) return 'playbook';
  if (under(pathname, '/informes')) return 'preparar-rival';
  if (under(pathname, '/integridad')) return 'partidas';
  const section = sectionForPath(pathname);
  if (section === null) return null;
  const all = [...SIDEBAR_GROUPS.flatMap((group) => group.entries), ...SIDEBAR_FOOTER_ENTRIES];
  return all.find((entry) => entry.href === section.href)?.id ?? null;
}
