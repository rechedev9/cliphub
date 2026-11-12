import { NextResponse, type NextRequest } from 'next/server.js';
import { localAPIRequestError } from './lib/api/local-request-guard.ts';
import { currentEdition, isPathAvailable } from './lib/edition.ts';

/** The API routes this proxy guards; the rest check the guard in-handler. */
const PROXY_GUARD_SCOPE = /^\/api\/(?!(?:demos\/scan\/?|library\/demos\/?|library\/import-package\/?|library\/matches\/[^/]+\/voice\/?|library\/matches\/[^/]+\/replay-video\/?|streams\/?|editor\/assets\/?|full-demo\/overlay-images\/?|session\/bootstrap\/?|voice-plugin\/gamestate\/?)$).*/;

function isAPI(pathname: string): boolean {
  return pathname === '/api' || pathname.startsWith('/api/');
}

/** Rejects cross-origin and DNS-rebound access to every local API endpoint. */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  const { pathname } = request.nextUrl;
  const edition = currentEdition();

  if (edition === 'web' && !isPathAvailable(pathname, edition)) {
    if (isAPI(pathname)) return NextResponse.json({ error: 'not available in this edition' }, { status: 404 });
    const url = request.nextUrl.clone();
    url.pathname = '/descargar';
    url.search = '';
    return NextResponse.redirect(url, 307);
  }

  if (!PROXY_GUARD_SCOPE.test(pathname)) return NextResponse.next();

  const error = await localAPIRequestError(request.headers, request.method);
  if (error === undefined) return NextResponse.next();
  return NextResponse.json({ error }, { status: 403 });
}

export const config = {
  // Static assets skip the edition check; the API matcher keeps its own exclusions in the proxy.
  matcher: [
    '/api/:path*',
    '/((?!api|_next/|favicon|icon\\.svg|.*\\..*).*)',
  ],
};
