import { NextResponse, type NextRequest } from 'next/server.js';
import { localAPIRequestError } from './lib/api/local-request-guard.ts';
import { clearedCookies, resolveSession, sessionCookies, ACCESS_COOKIE, ACCESS_HEADER, REFRESH_COOKIE, type CookieSpec } from './lib/clipmind/session.ts';
import { currentEdition, isPathAvailable, isPublicWebPath } from './lib/edition.ts';

/** The API routes this proxy guards; the rest check the guard in-handler. */
const PROXY_GUARD_SCOPE = /^\/api\/(?!(?:demos\/scan\/?|library\/demos\/?|library\/import-package\/?|library\/matches\/[^/]+\/voice\/?|library\/matches\/[^/]+\/replay-video\/?|streams\/?|editor\/assets\/?|full-demo\/overlay-images\/?|session\/bootstrap\/?|voice-plugin\/gamestate\/?)$).*/;

function isAPI(pathname: string): boolean {
  return pathname === '/api' || pathname.startsWith('/api/');
}

function applyCookies(response: NextResponse, specs: CookieSpec[]): void {
  for (const spec of specs) response.cookies.set(spec.name, spec.value, spec.options);
}

/** Rejects cross-origin and DNS-rebound access to every local API endpoint. */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  const { pathname } = request.nextUrl;
  const edition = currentEdition();

  if (!isPathAvailable(pathname, edition)) {
    if (isAPI(pathname)) return NextResponse.json({ error: 'not available in this edition' }, { status: 404 });
    const url = request.nextUrl.clone();
    url.pathname = edition === 'web' ? '/descargar' : '/';
    url.search = '';
    return NextResponse.redirect(url, 307);
  }

  if (edition === 'web') return webEdition(request);

  if (!PROXY_GUARD_SCOPE.test(pathname)) return NextResponse.next();

  const error = await localAPIRequestError(request.headers, request.method);
  if (error === undefined) return NextResponse.next();
  return NextResponse.json({ error }, { status: 403 });
}

/**
 * The web edition: every page and API asks for a sign-in except the account
 * forms. A live access token travels to the route handlers in a header the
 * browser cannot set, and a refreshed one goes back in cookies.
 */
async function webEdition(request: NextRequest): Promise<NextResponse> {
  const { pathname, search } = request.nextUrl;
  const secure = request.nextUrl.protocol === 'https:' || request.headers.get('x-forwarded-proto') === 'https';

  const originError = await localAPIRequestError(request.headers, request.method);
  if (originError !== undefined) return NextResponse.json({ error: originError }, { status: 403 });

  const forwarded = new Headers(request.headers);
  forwarded.delete(ACCESS_HEADER);
  if (isPublicWebPath(pathname)) return NextResponse.next({ request: { headers: forwarded } });

  const session = await resolveSession(request.cookies);
  if (session === undefined) {
    let response: NextResponse;
    if (isAPI(pathname)) {
      response = NextResponse.json({ error: 'sign in required', code: 'unauthorized' }, { status: 401 });
    } else {
      const url = request.nextUrl.clone();
      url.pathname = '/iniciar-sesion';
      url.search = `?next=${encodeURIComponent(`${pathname}${search}`)}`;
      response = NextResponse.redirect(url, 307);
    }
    // A sign-in that no longer works is dropped, so the next visit starts clean.
    if (request.cookies.has(ACCESS_COOKIE) || request.cookies.has(REFRESH_COOKIE)) applyCookies(response, clearedCookies(secure));
    return response;
  }
  forwarded.set(ACCESS_HEADER, session.access);
  const response = NextResponse.next({ request: { headers: forwarded } });
  if (session.renewed !== undefined) applyCookies(response, sessionCookies(session.renewed, secure));
  return response;
}

export const config = {
  // Static assets skip the edition check; the API matcher keeps its own exclusions in the proxy.
  matcher: [
    '/api/:path*',
    '/((?!api|_next/|favicon|icon\\.svg|.*\\..*).*)',
  ],
};
