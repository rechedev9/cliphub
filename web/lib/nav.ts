/** Numbered Studio nav. The index is the rail order, padded to two digits. */
export const NAV_SECTIONS = [
  { number: '01', label: 'Clips y vídeos', href: '/clips' },
  { number: '02', label: 'Stream clips', href: '/streams' },
  { number: '03', label: 'Players', href: '/players' },
  { number: '04', label: 'Táctica', href: '/tactical' },
  { number: '05', label: 'Anti-cheat', href: '/cheaters' },
  { number: '06', label: 'Ajustes', href: '/settings' },
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
