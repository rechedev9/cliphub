import { sectionForPath, type NavHref } from './nav.ts';

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
  | 'libreta'
  | 'lineups-pro'
  | 'simulacion-de-veto'
  | 'preparar-rival'
  | 'informes'
  | 'integridad'
  | 'almacenamiento'
  | 'ajustes';

export type SidebarEntry = {
  id: SidebarEntryId;
  label: string;
  href: `/${string}`;
  /** True while the screen has no route of its own: the entry opens the closest screen and never lights up. */
  temporary?: true;
};

export type SidebarGroup = { id: string; label: string; entries: readonly SidebarEntry[] };

/** Rail groups in order. Group labels render as `/ LABEL`; every NAV_SECTIONS destination appears once as a real entry. */
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
      { id: 'replay', label: 'Replay 2D', href: '/replay' },
      { id: 'buscador-de-patrones', label: 'Buscador de patrones', href: '/equipos', temporary: true },
    ],
  },
  {
    id: 'estadisticas',
    label: 'Estadísticas',
    entries: [
      { id: 'estadisticas-de-equipo', label: 'Estadísticas de equipo', href: '/equipos' },
      { id: 'estadisticas-de-jugador', label: 'Estadísticas de jugador', href: '/jugadores', temporary: true },
      { id: 'ranking', label: 'Ranking', href: '/ranking' },
    ],
  },
  {
    id: 'playbook',
    label: 'Playbook',
    entries: [
      { id: 'editor-de-tacticas', label: 'Editor de tácticas', href: '/libreta', temporary: true },
      { id: 'playbook', label: 'Playbook', href: '/equipos', temporary: true },
      { id: 'libreta', label: 'Libreta', href: '/libreta' },
      { id: 'lineups-pro', label: 'Lineups pro', href: '/lineups' },
    ],
  },
  {
    id: 'otras-herramientas',
    label: 'Otras herramientas',
    entries: [
      { id: 'simulacion-de-veto', label: 'Simulación de veto', href: '/partidas/series', temporary: true },
      { id: 'preparar-rival', label: 'Preparar rival', href: '/equipos', temporary: true },
      { id: 'informes', label: 'Informes', href: '/informes' },
      { id: 'integridad', label: 'Integridad', href: '/integridad' },
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

/** The rail section a pathname lives in, so only a real entry of that section is active. */
export function activeSidebarHref(pathname: string): NavHref | null {
  return sectionForPath(pathname)?.href ?? null;
}

export function isSidebarEntryActive(entry: SidebarEntry, activeHref: NavHref | null): boolean {
  return entry.temporary === undefined && entry.href === activeHref;
}
