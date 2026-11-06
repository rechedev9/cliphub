import { NextResponse } from 'next/server';
import { RADAR_MAP_NAME_RE, isRadarLayer } from '@/lib/api/game-assets';
import { orchestratorUrl, proxyGameAsset } from '../../../demos/_lib';

export const runtime = 'nodejs';

/**
 * GET /api/maps/{map}/radar?layer=upper|lower: the map's overview radar as PNG, decoded from the
 * local CS2 install. proxy.ts runs the local-request guard.
 */
export async function GET(request: Request, { params }: { params: Promise<{ map: string }> }): Promise<Response> {
  const { map } = await params;
  if (!RADAR_MAP_NAME_RE.test(map)) {
    return NextResponse.json({ error: 'invalid map name', code: 'invalid_request' }, { status: 400 });
  }
  const layer = new URL(request.url).searchParams.get('layer') ?? 'upper';
  if (!isRadarLayer(layer)) {
    return NextResponse.json({ error: 'layer must be upper or lower', code: 'invalid_request' }, { status: 400 });
  }
  return proxyGameAsset(`${orchestratorUrl()}/api/maps/${map}/radar?layer=${layer}`);
}
