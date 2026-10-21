/** Studio nav in rail order. */
export const NAV_SECTIONS = [
  { label: 'Shorts', href: '/shorts' },
  { label: 'Vídeos largos', href: '/videos-largos' },
  { label: 'Clips de stream', href: '/streams' },
  { label: 'Resultados', href: '/resultados' },
  { label: 'Jugadores', href: '/players' },
  { label: 'Táctica', href: '/tactical' },
  { label: 'Anti-cheat', href: '/cheaters' },
  { label: 'Algoritmos', href: '/algoritmos' },
  { label: 'Nube', href: '/nube' },
  { label: 'Ajustes', href: '/settings' },
] as const;

export type NavSection = (typeof NAV_SECTIONS)[number];
export type NavHref = NavSection['href'];

/**
 * Retired sections and where they landed. The single hub split into one flow
 * per format plus the results area, so every old door redirects to one of them.
 */
export const RETIRED_ROUTES = {
  '/clips': '/resultados',
  '/onboarding': '/resultados',
  '/matches': '/shorts',
  '/upload': '/videos-largos',
  '/full-demo': '/videos-largos',
  '/editor': '/resultados',
  '/videos': '/resultados',
  '/feed': '/resultados',
} as const;
