import { NextResponse, type NextRequest } from 'next/server.js';
import { localAPIRequestError } from './lib/api/local-request-guard.ts';

/** Rejects cross-origin and DNS-rebound access to every local API endpoint. */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  const error = await localAPIRequestError(request.headers, request.method);
  if (error === undefined) return NextResponse.next();
  return NextResponse.json({ error }, { status: 403 });
}

export const config = {
  // Large uploads check the guard in-handler so Next does not buffer the body.
  matcher: '/api/((?!demos/scan/?$|streams/?$|session/bootstrap/?$).*)',
};
