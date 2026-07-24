/**
 * Single source of truth for the numbered Studio nav sections. The sidebar
 * and every StudioPageHeader derive their `// 0N — LABEL` numbering from this
 * ordered list, so a reorder here renumbers the whole app consistently.
 * Icons stay in the sidebar component; this module is data only.
 *
 * `startsGroup` marks where creation destinations end and content destinations
 * begin. It lives here, on the entry, because the separator belongs to the
 * order: inserting a section used to leave the sidebar's hardcoded divider
 * index pointing at the wrong row.
 */
export const NAV_SECTIONS = [
  { number: '01', label: 'Partidas', href: '/matches', startsGroup: false },
  { number: '02', label: 'Subir demo', href: '/upload', startsGroup: false },
  { number: '03', label: 'Táctica', href: '/tactical', startsGroup: false },
  { number: '04', label: 'Clips de stream', href: '/streams', startsGroup: false },
  { number: '05', label: 'Noticias', href: '/news', startsGroup: false },
  { number: '06', label: 'Biblioteca', href: '/videos', startsGroup: true },
  { number: '07', label: 'Feed', href: '/feed', startsGroup: false },
  { number: '08', label: 'Ajustes', href: '/settings', startsGroup: false },
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
