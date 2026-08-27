import { NextResponse, type NextRequest } from 'next/server.js';
import { localAPICORSHeaders, localAPIRequestError } from './lib/api/local-request-guard.ts';
import { configuredControlPlaneURL, isHostedPublicPath, isHostedWebMode } from './lib/hosted-mode.ts';

/** Rejects cross-origin and DNS-rebound access to every local API endpoint. */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  if (isHostedWebMode()) return hostedRequest(request);
  if (!request.nextUrl.pathname.startsWith('/api/')) return NextResponse.next();
  const cors = localAPICORSHeaders(request.headers);
  if (request.method === 'OPTIONS' && cors !== null) {
    return new NextResponse(null, { status: 204, headers: cors });
  }
  const error = await localAPIRequestError(request.headers, request.method);
  if (error === undefined) {
    const response = NextResponse.next();
    cors?.forEach((value, name) => response.headers.set(name, value));
    return response;
  }
  return NextResponse.json({ error }, { status: 403 });
}

async function hostedRequest(request: NextRequest): Promise<NextResponse> {
  const pathname = request.nextUrl.pathname;
  if (isHostedPublicPath(pathname)) return NextResponse.next();
  if (pathname.startsWith('/api/')) {
    return NextResponse.json(
      { code: 'agent_required', error: 'Conecta ClipHub Agent para usar las herramientas locales.' },
      { status: 503 },
    );
  }
  const controlURL = configuredControlPlaneURL();
  if (controlURL === null) return NextResponse.redirect(new URL('/login?error=service', request.url));
  let response: Response;
  try {
    response = await fetch(new URL('/api/account/session', controlURL), {
      headers: { Cookie: request.headers.get('cookie') ?? '' },
      cache: 'no-store',
      redirect: 'manual',
    });
  } catch {
    return NextResponse.redirect(new URL('/login?error=service', request.url));
  }
  if (response.ok) return NextResponse.next();
  const login = new URL('/login', request.url);
  login.searchParams.set('next', `${pathname}${request.nextUrl.search}`);
  return NextResponse.redirect(login);
}

export const config = {
  // Large local uploads check the guard in-handler so Next does not buffer the body.
  matcher: [
    '/((?!api/|_next/static|_next/image|favicon.ico).*)',
    '/api/((?!demos/scan/?$|streams/?$|editor/assets/?$|session/bootstrap/?$).*)',
  ],
};
