import { NextResponse } from 'next/server';
import { orchestratorUrl, callOrchestrator, serviceUnavailable, forwardError } from '../../demos/_lib';
import { whitelistSteamAccount, type UpstreamSteamAccount } from '../_lib';

export const runtime = 'nodejs';

export async function GET(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/steam/account`);
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as UpstreamSteamAccount;
  return NextResponse.json(whitelistSteamAccount(data), { headers: { 'cache-control': 'no-store' } });
}

export async function PUT(request: Request): Promise<Response> {
  const raw = (await request.json().catch(() => null)) as {
    steamId?: unknown;
    authCode?: unknown;
    apiKey?: unknown;
    knownCode?: unknown;
  } | null;
  const res = await callOrchestrator(`${orchestratorUrl()}/api/steam/account`, {
    method: 'PUT',
    headers: { 'content-type': 'application/json' },
    body: JSON.stringify({
      steamId: typeof raw?.steamId === 'string' ? raw.steamId : '',
      authCode: typeof raw?.authCode === 'string' ? raw.authCode : '',
      apiKey: typeof raw?.apiKey === 'string' ? raw.apiKey : '',
      knownCode: typeof raw?.knownCode === 'string' ? raw.knownCode : '',
    }),
  });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as UpstreamSteamAccount;
  return NextResponse.json(whitelistSteamAccount(data), { headers: { 'cache-control': 'no-store' } });
}

export async function DELETE(): Promise<Response> {
  const res = await callOrchestrator(`${orchestratorUrl()}/api/steam/account`, { method: 'DELETE' });
  if (res === null) return serviceUnavailable();
  if (!res.ok) return forwardError(res);
  const data = (await res.json()) as UpstreamSteamAccount;
  return NextResponse.json(whitelistSteamAccount(data), { headers: { 'cache-control': 'no-store' } });
}
