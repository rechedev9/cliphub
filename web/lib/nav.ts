/** Numbered Studio nav. Inicio is `00` so the other section numbers stay put. */
export const NAV_SECTIONS = [
  { number: '00', label: 'Inicio', href: '/onboarding' },
  { number: '01', label: 'Partidas', href: '/matches' },
  { number: '02', label: 'Subir demo', href: '/upload' },
  { number: '12', label: 'Full demo to video', href: '/full-demo' },
  { number: '03', label: 'Táctica', href: '/tactical' },
  { number: '04', label: 'CheaterDetect', href: '/cheaters' },
  { number: '05', label: 'Jugadores', href: '/players' },
  { number: '06', label: 'Clips de stream', href: '/streams' },
  { number: '07', label: 'Noticias', href: '/news' },
  { number: '08', label: 'Editor', href: '/editor' },
  { number: '09', label: 'Biblioteca', href: '/videos' },
  { number: '10', label: 'Feed', href: '/feed' },
  { number: '11', label: 'Ajustes', href: '/settings' },
] as const;

export type NavSection = (typeof NAV_SECTIONS)[number];
export type NavHref = NavSection['href'];

/** The nav entry for a known Studio href; unknown hrefs fail at compile time. */
export function navSection(href: NavHref): NavSection {
  const entry = NAV_SECTIONS.find((section) => section.href === href);
  if (entry === undefined) {
    throw new Error(`unknown nav section: ${href}`);
  }
  return entry;
}
