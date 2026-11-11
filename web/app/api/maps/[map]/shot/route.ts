import { NextResponse } from 'next/server';
import { RADAR_MAP_NAME_RE } from '@/lib/api/game-assets';
import { orchestratorUrl, proxyGameAsset } from '../../../demos/_lib';

export const runtime = 'nodejs';

/** GET /api/maps/{map}/shot: the map's loading-screen picture as PNG, from the local CS2 install. */
export async function GET(_request: Request, { params }: { params: Promise<{ map: string }> }): Promise<Response> {
  const { map } = await params;
  if (!RADAR_MAP_NAME_RE.test(map)) {
    return NextResponse.json({ error: 'invalid map name', code: 'invalid_request' }, { status: 400 });
  }
  return proxyGameAsset(`${orchestratorUrl()}/api/maps/${map}/shot`);
}
