import { NextResponse } from 'next/server';
import { TEAM_LOGO_NAME_MAX } from '@/lib/api/game-assets';
import { orchestratorUrl, proxyGameAsset } from '../../demos/_lib';

export const runtime = 'nodejs';

/** GET /api/teams/logo?name=: a team's Major sticker logo as SVG, from the local CS2 install. */
export async function GET(request: Request): Promise<Response> {
  const name = new URL(request.url).searchParams.get('name') ?? '';
  if (name === '' || [...name].length > TEAM_LOGO_NAME_MAX) {
    return NextResponse.json({ error: `name must hold 1 to ${TEAM_LOGO_NAME_MAX} characters`, code: 'invalid_request' }, { status: 400 });
  }
  return proxyGameAsset(`${orchestratorUrl()}/api/teams/logo?name=${encodeURIComponent(name)}`);
}
