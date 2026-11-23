/** The two work areas of the rail; `chrome` docks to the footer without a heading. */
export type NavArea = 'create' | 'analyze' | 'chrome';

export const NAV_AREA_LABEL = {
  create: 'Crear',
  analyze: 'Analizar',
} as const satisfies Record<Exclude<NavArea, 'chrome'>, string>;

/** Studio nav in rail order: the one source of the section names the rail, the breadcrumb and the guide show. */
export const NAV_SECTIONS = [
  { label: 'Demo completa', href: '/videos-largos', area: 'create' },
  { label: 'Short', href: '/shorts', area: 'create' },
  { label: 'Clips de stream', href: '/streams', area: 'create' },
  { label: 'Mis vídeos', href: '/mis-videos', area: 'create' },
  { label: 'Consejos', href: '/ideas', area: 'create' },
  { label: 'Partidas', href: '/partidas', area: 'analyze' },
  { label: 'Replay 2D', href: '/replay', area: 'analyze' },
  { label: 'Buscador de patrones', href: '/patrones', area: 'analyze' },
  { label: 'Equipos', href: '/equipos', area: 'analyze' },
  { label: 'Jugadores', href: '/jugadores', area: 'analyze' },
  { label: 'Jugadores FACEIT', href: '/jugadores/faceit', area: 'analyze' },
  { label: 'Ranking', href: '/ranking', area: 'analyze' },
  { label: 'Libreta', href: '/libreta', area: 'analyze' },
  { label: 'Editor de tácticas', href: '/tacticas/nueva', area: 'analyze' },
  { label: 'Lineups pro', href: '/lineups', area: 'analyze' },
  { label: 'Informes', href: '/informes', area: 'analyze' },
  { label: 'Almacenamiento', href: '/almacenamiento', area: 'chrome' },
  { label: 'Ajustes', href: '/settings', area: 'chrome' },
] as const satisfies readonly { label: string; href: `/${string}`; area: NavArea }[];

export type NavSection = (typeof NAV_SECTIONS)[number];
export type NavHref = NavSection['href'];

/** Routes outside the rail that belong to a section: a Short series (Demo completa ones live under /videos-largos). */
const SECTION_PREFIXES: ReadonlyArray<readonly [prefix: string, href: NavHref]> = [
  ['/series/', '/shorts'],
];

export function navSection(href: NavHref): NavSection {
  const section = NAV_SECTIONS.find((entry) => entry.href === href);
  if (section === undefined) throw new Error(`${href} is not a rail section`);
  return section;
}

function inSection(pathname: string, href: string): boolean {
  return pathname === href || pathname.startsWith(`${href}/`);
}

/** The rail section a pathname lives in, including nested routes; the deepest section wins. */
export function sectionForPath(pathname: string): NavSection | null {
  const owner = SECTION_PREFIXES.find(([prefix]) => pathname.startsWith(prefix))?.[1];
  if (owner !== undefined) return navSection(owner);
  let found: NavSection | null = null;
  for (const section of NAV_SECTIONS) {
    if (inSection(pathname, section.href) && (found === null || section.href.length > found.href.length)) found = section;
  }
  return found;
}

/** Tool pages bring their own context (the team and the filters), so the strip leaves the breadcrumb off them. */
export function isToolPage(pathname: string): boolean {
  return pathname === '/replay' || ['/patrones', '/tacticas'].some((root) => pathname === root || pathname.startsWith(`${root}/`));
}

/**
 * Retired sections and where they landed. Every old door redirects; the
 * renamed sections also carry their nested paths and query along.
 */
export const RETIRED_ROUTES = {
  '/clips': '/mis-videos',
  '/onboarding': '/mis-videos',
  '/matches': '/shorts',
  '/upload': '/videos-largos',
  '/full-demo': '/videos-largos',
  '/editor': '/mis-videos',
  '/videos': '/mis-videos',
  '/feed': '/mis-videos',
  '/nube': '/mis-videos',
  '/resultados': '/mis-videos',
  '/algoritmos': '/ideas',
  '/players': '/jugadores/faceit',
  '/tactical': '/partidas',
  '/stratbook': '/libreta',
} as const;

export type RetiredRoute = keyof typeof RETIRED_ROUTES;
type SearchValue = string | readonly string[] | undefined;

// Next hands catch-all segments over still percent-encoded; decoding first keeps them from being encoded twice.
function decodeSegment(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
}

/** Where a retired URL lands, keeping its nested segments and its query string. */
export function retiredHref(
  from: RetiredRoute,
  request: { rest?: readonly string[]; search?: Readonly<Record<string, SearchValue>> } = {},
): string {
  const rest = (request.rest ?? []).map((segment) => `/${encodeURIComponent(decodeSegment(segment))}`).join('');
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(request.search ?? {})) {
    if (value === undefined) continue;
    for (const entry of typeof value === 'string' ? [value] : value) params.append(key, entry);
  }
  const query = params.toString();
  return `${RETIRED_ROUTES[from]}${rest}${query === '' ? '' : `?${query}`}`;
}
