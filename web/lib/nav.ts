/** Studio nav in rail order. */
export const NAV_SECTIONS = [
  { label: 'Clips y vídeos', href: '/clips' },
  { label: 'Clips de stream', href: '/streams' },
  { label: 'Jugadores', href: '/players' },
  { label: 'Táctica', href: '/tactical' },
  { label: 'Anti-cheat', href: '/cheaters' },
  { label: 'Ajustes', href: '/settings' },
] as const;

export type NavSection = (typeof NAV_SECTIONS)[number];
export type NavHref = NavSection['href'];

/**
 * Retired sections and where they landed. The demo pipeline collapsed into
 * `/clips` (partida = source of truth), so every old door redirects there.
 */
export const RETIRED_ROUTES = {
  '/onboarding': '/clips',
  '/matches': '/clips',
  '/upload': '/clips/nueva',
  '/full-demo': '/clips',
  '/editor': '/clips',
  '/videos': '/clips?vista=clips',
  '/feed': '/clips?vista=clips',
} as const;
