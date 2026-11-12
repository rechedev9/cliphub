import { RESULTS_HREF } from './clips/routes.ts';
import { NAV_SECTIONS, RETIRED_ROUTES, sectionForPath, type NavSection } from './nav.ts';

export type Edition = 'desktop' | 'web';

const DESKTOP_PAGE_ROUTES = [
  '/videos-largos',
  '/shorts',
  '/streams',
  '/jugadores/faceit',
  '/mis-videos',
  '/ideas',
  '/series',
  '/almacenamiento',
  '/modo-partida',
  '/upload',
] as const;

const DESKTOP_API_PREFIXES = [
  '/api/editor',
  '/api/full-demo',
  '/api/streams',
  '/api/imported-videos',
  '/api/songs',
  '/api/presets',
  '/api/practice',
  '/api/voice-plugin',
  '/api/steam',
  '/api/demos',
  '/api/clipmind',
] as const;

function matchesPrefix(pathname: string, prefix: string): boolean {
  return pathname === prefix || pathname.startsWith(`${prefix}/`);
}

const WEB_ONLY_PAGE_ROUTES = [
  '/iniciar-sesion',
  '/crear-cuenta',
  '/confirmar',
  '/recuperar',
  '/restablecer',
  '/invitacion',
  '/equipo',
  '/cuenta',
  '/importar',
  '/api/cm',
] as const;

/** Reachable without a sign-in in the web edition; everything else asks for one. */
const PUBLIC_WEB_ROUTES: readonly string[] = [
  '/iniciar-sesion',
  '/crear-cuenta',
  '/confirmar',
  '/recuperar',
  '/restablecer',
  '/descargar',
  '/api/cm/auth/signup',
  '/api/cm/auth/confirm',
  '/api/cm/auth/login',
  '/api/cm/auth/forgot',
  '/api/cm/auth/reset',
];

const DESKTOP_RETIRED_ROUTES = Object.entries(RETIRED_ROUTES)
  .filter(([, target]) => DESKTOP_PAGE_ROUTES.some((route) => matchesPrefix(target, route)))
  .map(([from]) => from);

const DESKTOP_ONLY_PREFIXES: readonly string[] = [
  ...DESKTOP_PAGE_ROUTES,
  ...DESKTOP_RETIRED_ROUTES,
  ...DESKTOP_API_PREFIXES,
];

/** Desktop is the default so existing Electron builds keep every route. */
export function currentEdition(): Edition {
  return process.env.NEXT_PUBLIC_CLIPHUB_EDITION === 'web' ? 'web' : 'desktop';
}

export function isPathAvailable(pathname: string, edition: Edition): boolean {
  if (edition === 'desktop') return !WEB_ONLY_PAGE_ROUTES.some((prefix) => matchesPrefix(pathname, prefix));
  return !DESKTOP_ONLY_PREFIXES.some((prefix) => matchesPrefix(pathname, prefix));
}

export function isPublicWebPath(pathname: string): boolean {
  return PUBLIC_WEB_ROUTES.includes(pathname.length > 1 ? pathname.replace(/\/$/, '') : pathname);
}

export function homePathFor(edition: Edition): string {
  return edition === 'web' ? '/partidas' : RESULTS_HREF;
}

export function homeLabelFor(edition: Edition): string {
  return edition === 'web' ? 'Partidas' : 'Mis vídeos';
}

export function navSectionsFor(edition: Edition): readonly NavSection[] {
  return NAV_SECTIONS.filter((section) => isPathAvailable(section.href, edition));
}

/** The section a path belongs to, only when the edition serves it. */
export function editionSectionForPath(pathname: string, edition: Edition): NavSection | null {
  const section = sectionForPath(pathname);
  return section !== null && isPathAvailable(section.href, edition) ? section : null;
}
