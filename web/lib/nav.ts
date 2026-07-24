/**
 * Single source of truth for the numbered Studio nav sections. The sidebar
 * and every StudioPageHeader derive their `// 0N — LABEL` numbering from this
 * ordered list, so a reorder here renumbers the whole app consistently.
 * Icons stay in the sidebar component; this module is data only.
 *
 * Grouping is not here: the sidebar's NAV_META assigns each section to a phase
 * of the Studio flow, which is what replaced the hardcoded divider index that
 * used to point at the wrong row whenever anyone inserted a section.
 */
export const NAV_SECTIONS = [
  { number: '01', label: 'Partidas', href: '/matches' },
  { number: '02', label: 'Subir demo', href: '/upload' },
  { number: '03', label: 'Táctica', href: '/tactical' },
  { number: '04', label: 'CheaterDetect', href: '/cheaters' },
  { number: '05', label: 'Clips de stream', href: '/streams' },
  { number: '06', label: 'Noticias', href: '/news' },
  { number: '07', label: 'Biblioteca', href: '/videos' },
  { number: '08', label: 'Feed', href: '/feed' },
  { number: '09', label: 'Ajustes', href: '/settings' },
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
