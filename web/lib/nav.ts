/** Numbered Studio nav. The index is the rail order, padded to two digits. */
export const NAV_SECTIONS = [
  { number: '00', label: 'Inicio', href: '/onboarding' },
  { number: '01', label: 'Partidas', href: '/matches' },
  { number: '02', label: 'Subir demo', href: '/upload' },
  { number: '03', label: 'Full demo to video', href: '/full-demo' },
  { number: '04', label: 'Táctica', href: '/tactical' },
  { number: '05', label: 'CheaterDetect', href: '/cheaters' },
  { number: '06', label: 'Jugadores', href: '/players' },
  { number: '07', label: 'Clips de stream', href: '/streams' },
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
