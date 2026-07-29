import { NextResponse } from 'next/server';
import { readBoundedText } from '@/lib/api/bounded-request-body';
import {
  BOOTSTRAP_CAPABILITY_ERROR,
  localAPIOrigin,
  localAPIBootstrapError,
  PROXY_BOOTSTRAP_CAPABILITY_ENV,
  PROXY_MUTATION_CAPABILITY_ENV,
  PROXY_MUTATION_CAPABILITY_COOKIE,
} from '@/lib/api/local-request-guard';

export const runtime = 'nodejs';

const FORM_CONTENT_TYPE = 'application/x-www-form-urlencoded';

function bootstrapErrorRedirect(request: Request, code: 'capability' | 'unavailable'): Response {
  return NextResponse.redirect(new URL(`/bootstrap?error=${code}`, request.url), 303);
}

/**
 * POST /api/session/bootstrap seeds the HttpOnly mutation-capability cookie
 * for a standalone browser launch. Electron seeds the same cookie directly
 * through session.cookies and never needs this route.
 */
export async function POST(request: Request): Promise<Response> {
  if (!request.headers.get('content-type')?.toLowerCase().startsWith(FORM_CONTENT_TYPE)) {
    return NextResponse.json({ error: 'form body required' }, { status: 400 });
  }

  const body = await readBoundedText(request, 4 * 1024);
  if (!body.ok) return NextResponse.json({ error: body.error }, { status: body.status });

  const capability = new URLSearchParams(body.text).get('capability');
  if (capability === null) {
    return bootstrapErrorRedirect(request, 'capability');
  }
  if (!process.env[PROXY_BOOTSTRAP_CAPABILITY_ENV]) {
    return bootstrapErrorRedirect(request, 'unavailable');
  }
  const error = await localAPIBootstrapError(request.headers, capability);
  if (error !== undefined) {
    return bootstrapErrorRedirect(request, error === BOOTSTRAP_CAPABILITY_ERROR ? 'capability' : 'unavailable');
  }

  const proxyCapability = process.env[PROXY_MUTATION_CAPABILITY_ENV];
  if (!proxyCapability) return bootstrapErrorRedirect(request, 'unavailable');

  // The guard above has validated Host as an explicit loopback origin. Keep
  // that exact host so a cookie seeded on 127.0.0.1 is not lost by redirecting
  // to Next's canonical localhost Request.url (or vice versa).
  const localOrigin = localAPIOrigin(request.headers);
  if (!localOrigin) return bootstrapErrorRedirect(request, 'unavailable');
  const response = NextResponse.redirect(new URL('/upload', localOrigin), 303);
  response.cookies.set({
    name: PROXY_MUTATION_CAPABILITY_COOKIE,
    value: proxyCapability,
    httpOnly: true,
    sameSite: 'strict',
    path: '/',
    // The local standalone server uses HTTP loopback. Electron sets this cookie
    // directly with the equivalent origin/path constraints.
    secure: false,
  });
  return response;
}
