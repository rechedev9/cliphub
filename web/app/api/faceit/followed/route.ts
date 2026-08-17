import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';
import { whitelistFaceitPlayer, type UpstreamFaceitPlayer } from '../_lib';

export const runtime = 'nodejs';

export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/followed`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as { enabled?: unknown; players?: unknown };
  const players = Array.isArray(data.players)
    ? data.players
      .map((item) => whitelistFaceitPlayer(item as UpstreamFaceitPlayer))
      .filter((player) => player !== null)
    : [];
  return NextResponse.json(
    { enabled: data.enabled === true, players },
    { headers: { 'cache-control': 'no-store' } },
  );
}

export async function POST(request: Request): Promise<Response> {
  let nickname = '';
  try {
    const body = (await request.json()) as { nickname?: unknown };
    nickname = typeof body.nickname === 'string' ? body.nickname : '';
  } catch {
    return NextResponse.json({ error: 'invalid follow request JSON' }, { status: 400 });
  }
  if (nickname.trim() === '') {
    return NextResponse.json({ error: 'FACEIT nickname is required' }, { status: 400 });
  }
  const res = await callOrchestrator(`${orchestratorUrl()}/api/faceit/followed`, {
    method: 'POST',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({ nickname }),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as { player?: UpstreamFaceitPlayer };
  const player = whitelistFaceitPlayer(data.player);
  if (player === null) {
    return NextResponse.json({ error: 'FACEIT player response is invalid' }, { status: 502 });
  }
  return NextResponse.json({ player }, { headers: { 'cache-control': 'no-store' } });
}
