import { NextResponse, type NextRequest } from 'next/server.js';
import { localAPIRequestError } from './lib/api/local-request-guard.ts';
import { clearedCookies, resolveSession, sessionCookies, ACCESS_COOKIE, ACCESS_HEADER, REFRESH_COOKIE, type CookieSpec } from './lib/clipmind/session.ts';
import { DEMO_COOKIE, DEMO_PATH, demoCookieOptions, demoDecision, demoHome } from './lib/clipmind/demo.ts';
import { currentEdition, isPathAvailable, isPublicWebPath } from './lib/edition-routes.ts';

/** The API routes this proxy guards; the rest check the guard in-handler. */
const PROXY_GUARD_SCOPE = /^\/api\/(?!(?:demos\/scan\/?|library\/demos\/?|library\/uploads\/?|library\/import-package\/?|library\/matches\/[^/]+\/voice\/?|library\/matches\/[^/]+\/replay-video\/?|streams\/?|editor\/assets\/?|full-demo\/overlay-images\/?|demos\/[^/]+\/player-card\/image\/?|session\/bootstrap\/?|voice-plugin\/gamestate\/?)$).*/;

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

  const home = demoHome();
  const session = await resolveSession(request.cookies);
  if (session === undefined) {
    let response: NextResponse;
    if (pathname === DEMO_PATH) {
      response = redirectTo(request, home ?? '/crear-cuenta');
      if (home !== undefined) response.cookies.set(DEMO_COOKIE, '1', demoCookieOptions(secure));
    } else if (home !== undefined && request.cookies.get(DEMO_COOKIE)?.value === '1') {
      response = demoVisit(request, forwarded);
    } else if (isAPI(pathname)) {
      response = NextResponse.json({ error: 'sign in required', code: 'unauthorized' }, { status: 401 });
    } else {
      response = redirectTo(request, `/iniciar-sesion?next=${encodeURIComponent(`${pathname}${search}`)}`);
    }
    // A sign-in that no longer works is dropped, so the next visit starts clean.
    if (request.cookies.has(ACCESS_COOKIE) || request.cookies.has(REFRESH_COOKIE)) applyCookies(response, clearedCookies(secure));
    return response;
  }
  forwarded.set(ACCESS_HEADER, session.access);
  // Someone signed in has their own library, not the demo's.
  const response = pathname === DEMO_PATH ? redirectTo(request, '/') : NextResponse.next({ request: { headers: forwarded } });
  if (session.renewed !== undefined) applyCookies(response, sessionCookies(session.renewed, secure));
  // Signed in now, so the demo banner goes.
  if (request.cookies.has(DEMO_COOKIE)) response.cookies.delete(DEMO_COOKIE);
  return response;
}

/** A visitor of the public demo: reads go through, writes and account pages do not. */
function demoVisit(request: NextRequest, forwarded: Headers): NextResponse {
  const { pathname } = request.nextUrl;
  const decision = demoDecision(request.method, pathname);
  if (decision === 'read_only') {
    return NextResponse.json({ error: 'the demo is read-only', code: 'demo_read_only' }, { status: 403 });
  }
  if (decision === 'sign_up') {
    if (isAPI(pathname)) return NextResponse.json({ error: 'sign in required', code: 'unauthorized' }, { status: 401 });
    return redirectTo(request, '/crear-cuenta');
  }
  return NextResponse.next({ request: { headers: forwarded } });
}

/** A 307 to a same-origin path, which may carry its own query. */
function redirectTo(request: NextRequest, path: string): NextResponse {
  return NextResponse.redirect(new URL(path, request.nextUrl), 307);
}

export const config = {
  // Uploads stream past the proxy: Next cuts a body it hands the proxy to 10 MB.
  // Those handlers run the guard and read the session cookie themselves.
  matcher: [
    '/api/((?!demos/scan/?$|library/demos/?$|library/uploads/?$|library/import-package/?$|library/matches/[^/]+/voice/?$|library/matches/[^/]+/replay-video/?$|streams/?$|editor/assets/?$|full-demo/overlay-images/?$|demos/[^/]+/player-card/image/?$|session/bootstrap/?$|voice-plugin/gamestate/?$).*)',
    '/((?!api|_next/|favicon|icon\\.svg|.*\\..*).*)',
  ],
};
