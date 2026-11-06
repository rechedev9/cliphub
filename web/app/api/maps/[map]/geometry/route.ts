import { NextResponse } from 'next/server';
import { MAP_GEOMETRY_NAME_RE, isMapGeometrySource, parseMapGeometry } from '@/lib/api/map-geometry';
import { callOrchestrator, forwardError, orchestratorUrl, serviceUnavailable } from '../../../demos/_lib';

export const runtime = 'nodejs';

/**
 * GET /api/maps/{map}/geometry?source=physics|nav: the map's collision or floor mesh read from the
 * local CS2 install. proxy.ts runs the local-request guard.
 */
export async function GET(request: Request, { params }: { params: Promise<{ map: string }> }): Promise<Response> {
  const { map } = await params;
  if (!MAP_GEOMETRY_NAME_RE.test(map)) {
    return NextResponse.json({ error: 'invalid map name', code: 'invalid_request' }, { status: 400 });
  }
  const source = new URL(request.url).searchParams.get('source');
  if (source !== null && !isMapGeometrySource(source)) {
    return NextResponse.json({ error: 'source must be physics or nav', code: 'invalid_request' }, { status: 400 });
  }
  const query = source === null ? '' : `?source=${source}`;
  // A first physics read of a big map takes about a second; later reads come from the cache.
  const res = await callOrchestrator(`${orchestratorUrl()}/api/maps/${map}/geometry${query}`, { cache: 'no-store' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const text = await res.text();
  let body: unknown = null;
  try {
    body = JSON.parse(text);
  } catch {
    body = null;
  }
  // Validated, then forwarded as sent: the mesh is up to 1.5 MB of numbers.
  if (parseMapGeometry(body) === null) return NextResponse.json({ error: 'upstream error' }, { status: 502 });
  return new Response(text, { status: 200, headers: { 'content-type': 'application/json' } });
}
