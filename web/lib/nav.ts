/** The two work areas of the rail; `chrome` docks to the footer without a heading. */
export type NavArea = 'create' | 'analyze' | 'chrome';

export const NAV_AREA_LABEL = {
  create: 'Crear',
  analyze: 'Analizar',
} as const satisfies Record<Exclude<NavArea, 'chrome'>, string>;

/** Studio nav in rail order. */
export const NAV_SECTIONS = [
  { label: 'Vídeo largo', href: '/videos-largos', area: 'create' },
  { label: 'Short', href: '/shorts', area: 'create' },
  { label: 'Clips de stream', href: '/streams', area: 'create' },
  { label: 'Mis vídeos', href: '/mis-videos', area: 'create' },
  { label: 'Ideas', href: '/ideas', area: 'create' },
  { label: 'Partidas', href: '/partidas', area: 'analyze' },
  { label: 'Equipos', href: '/equipos', area: 'analyze' },
  { label: 'Jugadores', href: '/jugadores', area: 'analyze' },
  { label: 'Stratbook', href: '/stratbook', area: 'analyze' },
  { label: 'Informes', href: '/informes', area: 'analyze' },
  { label: 'Integridad', href: '/integridad', area: 'analyze' },
  { label: 'Ajustes', href: '/settings', area: 'chrome' },
] as const satisfies readonly { label: string; href: `/${string}`; area: NavArea }[];

export type NavSection = (typeof NAV_SECTIONS)[number];
export type NavHref = NavSection['href'];

/** Routes outside the rail that belong to a section: a series produces one Short per map. */
const SECTION_PREFIXES: ReadonlyArray<readonly [prefix: string, href: NavHref]> = [['/series/', '/shorts']];

/** The rail section a pathname lives in, including nested routes under it. */
export function sectionForPath(pathname: string): NavSection | null {
  const owner = SECTION_PREFIXES.find(([prefix]) => pathname.startsWith(prefix))?.[1];
  for (const section of NAV_SECTIONS) {
    if (owner === section.href) return section;
    if (pathname === section.href || pathname.startsWith(`${section.href}/`)) return section;
  }
  return null;
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
  '/cheaters': '/integridad',
  '/algoritmos': '/ideas',
  '/players': '/jugadores/faceit',
  '/tactical': '/partidas',
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
