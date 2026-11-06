import { NextResponse } from 'next/server';
import { MAP_GEOMETRY_NAME_RE } from '@/lib/api/map-geometry';
import { callOrchestrator, forwardError, orchestratorUrl, serviceUnavailable } from '../../../demos/_lib';

export const runtime = 'nodejs';

/**
 * GET /api/maps/{map}/scene: the map's render meshes, textures and baked light from the local CS2
 * install, as the binary scene file `lib/api/map-scene.ts` reads. Streamed: it weighs up to 200 MB.
 * proxy.ts runs the local-request guard.
 */
export async function GET(_request: Request, { params }: { params: Promise<{ map: string }> }): Promise<Response> {
  const { map } = await params;
  if (!MAP_GEOMETRY_NAME_RE.test(map)) {
    return NextResponse.json({ error: 'invalid map name', code: 'invalid_request' }, { status: 400 });
  }
  const res = await callOrchestrator(`${orchestratorUrl()}/api/maps/${map}/scene`, { cache: 'no-store' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  if (res.headers.get('content-type') !== 'application/octet-stream' || res.body === null) {
    return NextResponse.json({ error: 'upstream error' }, { status: 502 });
  }
  const headers: Record<string, string> = { 'content-type': 'application/octet-stream', 'cache-control': 'no-store', 'x-content-type-options': 'nosniff' };
  const length = res.headers.get('content-length');
  if (length !== null) headers['content-length'] = length;
  return new Response(res.body, { status: 200, headers });
}
