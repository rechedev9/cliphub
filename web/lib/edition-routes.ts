import { RETIRED_ROUTES } from './nav.ts';

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
  '/api/brand-kit',
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
