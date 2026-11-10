import { NextResponse, type NextRequest } from 'next/server.js';
import { localAPIRequestError } from './lib/api/local-request-guard.ts';

/** Rejects cross-origin and DNS-rebound access to every local API endpoint. */
export async function proxy(request: NextRequest): Promise<NextResponse> {
  const error = await localAPIRequestError(request.headers, request.method);
  if (error === undefined) return NextResponse.next();
  return NextResponse.json({ error }, { status: 403 });
}

export const config = {
  // Large uploads and CS2's game state (no capability cookie) check the guard in-handler.
  matcher: '/api/((?!demos/scan/?$|library/demos/?$|library/import-package/?$|library/matches/[^/]+/voice/?$|streams/?$|editor/assets/?$|full-demo/overlay-images/?$|session/bootstrap/?$|voice-plugin/gamestate/?$).*)',
};
