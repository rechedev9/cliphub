import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';
import { whitelistFaceitPlayer } from '../_lib';

export const runtime = 'nodejs';

export async function GET(request: Request): Promise<Response> {
  const nickname = new URL(request.url).searchParams.get('nickname') ?? '';
  if (nickname.trim() === '' || nickname.length > 256) {
    return NextResponse.json({ error: 'FACEIT nickname is required' }, { status: 400 });
  }
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/players?nickname=${encodeURIComponent(nickname)}`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as { player?: Parameters<typeof whitelistFaceitPlayer>[0] };
  const player = whitelistFaceitPlayer(data.player);
  if (player === null) {
    return NextResponse.json({ error: 'FACEIT player response is invalid' }, { status: 502 });
  }
  return NextResponse.json({ player }, { headers: { 'cache-control': 'no-store' } });
}
